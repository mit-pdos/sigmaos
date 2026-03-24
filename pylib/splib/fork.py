"""Fork support for SigmaOS Python procs.

The proc runs expensive imports/initialization and then calls fork_point().
At fork_point, the process becomes a Zygote: it connects to the procd-local
fork supervisor socket and waits for fork requests.

The supervisor sends child args + env updates over the socket. The zygote forks,
creates a fresh PID namespace for the child, applies the env updates, and then
returns the child args to user code.
"""

from __future__ import annotations

import ctypes
import ctypes.util
import gc
import json
import os
import socket
import struct
import time
from typing import Any

from splib.utils import log_spawn_latency


SIGMA_FORK_SOCK = "SIGMA_FORK_SOCK"
SIGMA_FORK_ZYGOTE_KEY = "SIGMA_FORK_ZYGOTE_KEY"


_libc = ctypes.CDLL("libc.so.6", use_errno=True)
_libc.syscall.restype = ctypes.c_long

_PyOS_BeforeFork = ctypes.pythonapi.PyOS_BeforeFork
_PyOS_BeforeFork.argtypes = []
_PyOS_BeforeFork.restype = None

_PyOS_AfterFork_Parent = ctypes.pythonapi.PyOS_AfterFork_Parent
_PyOS_AfterFork_Parent.argtypes = []
_PyOS_AfterFork_Parent.restype = None

_PyOS_AfterFork_Child = ctypes.pythonapi.PyOS_AfterFork_Child
_PyOS_AfterFork_Child.argtypes = []
_PyOS_AfterFork_Child.restype = None

_SYS_clone = 56

_CLONE_NEWPID = 0x20000000
_SIGCHLD = 17


def _clone(flags: int) -> int:
    """Acts exactly like os.fork(), but for the clone syscall."""
    _PyOS_BeforeFork()

    try:
        pid = _libc.syscall(_SYS_clone, flags, 0, 0, 0, 0)
    except Exception:
        _PyOS_AfterFork_Parent()
        raise

    if pid == 0:
        _PyOS_AfterFork_Child()
    else:
        _PyOS_AfterFork_Parent()

    if pid < 0:
        errno = ctypes.get_errno()
        raise OSError(errno, os.strerror(errno))

    return pid


def _read_msg(sock: socket.socket) -> dict[str, Any]:
    data = sock.recv(64 * 1024)
    if not data:
        raise EOFError("unexpected EOF")
    return json.loads(data.decode("utf-8"))


def _write_msg(sock: socket.socket, msg: dict[str, Any]) -> None:
    b = json.dumps(msg).encode("utf-8")
    sock.sendall(b)


def _write_msg_with_credentials(sock: socket.socket, msg: dict[str, Any]) -> None:
    b = json.dumps(msg).encode("utf-8")

    creds = struct.pack("iII", os.getpid(), os.getuid(), os.getgid())
    ancdata = [(socket.SOL_SOCKET, socket.SCM_CREDENTIALS, creds)]

    sock.sendmsg([b], ancdata)



def fork_point() -> list[str]:
    """Block until fork supervisor requests a child, then fork and return args."""

    gc.freeze()

    sock_path = os.environ.get(SIGMA_FORK_SOCK)
    if not sock_path:
        raise RuntimeError(f"{SIGMA_FORK_SOCK} is not set")

    zygote_key = os.environ.get(SIGMA_FORK_ZYGOTE_KEY)
    if not zygote_key:
        raise RuntimeError(f"{SIGMA_FORK_ZYGOTE_KEY} is not set")

    # Persistent connection from the zygote to the supervisor.
    zsock = socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET)
    zsock.connect(sock_path)
    _write_msg(zsock, {"type": "hello", "zygote_key": zygote_key})
    resp = _read_msg(zsock)
    if resp.get("type") != "ok":
        raise RuntimeError(f"fork supervisor rejected hello: {resp}")

    z_sig_pid = os.environ.get("SIGMADEBUGPID", "")

    while True:
        try:
            msg = _read_msg(zsock)
        except EOFError:
            # Supervisor closed the connection -> shutdown zygote.
            exit(0)

        if msg.get("type") != "fork":
            continue

        start = time.time_ns()
        pid = _clone(_CLONE_NEWPID | _SIGCHLD)
        if pid != 0:
            # Parent: continue servicing future fork requests.
            continue
        log_spawn_latency("splib.fork.fork_point clone", pid=z_sig_pid, op_start=start, spawn_time=0)

        req_id = msg.get("req_id")
        env = msg.get("env") or []
        args = msg.get("args") or []

        # Notify supervisor that the child exists (peercred conveys host PID).
        # Can possibly be replaced by using SCM_CREDENTIALS
        start = time.time_ns()
        _write_msg_with_credentials(zsock, {"type": "child", "req_id": req_id})
        log_spawn_latency("splib.fork.fork_point notify supervisor", pid=z_sig_pid, op_start=start, spawn_time=0)

        # Apply env updates for this child proc.
        start = time.time_ns()
        for entry in env:
            s = str(entry)
            if "=" not in s:
                continue
            k, v = s.split("=", 1)
            os.environ[k] = v
        log_spawn_latency("splib.fork.fork_point apply env", pid=z_sig_pid, op_start=start, spawn_time=0)

        gc.enable()
        return [str(a) for a in args]


# Reduce memory overhead after forking.
gc.disable()
