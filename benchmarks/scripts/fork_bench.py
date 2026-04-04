from __future__ import annotations

import gc
gc.disable()

import ctypes
import ctypes.util
import os
import time


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


if __name__ == "__main__":
    gc.freeze()
    ncpu = os.cpu_count()
    per_cpu_procs = 1000

    pipes = [os.pipe() for _ in range(ncpu)]

    workers = []
    worker_index = 0
    is_worker = False

    for i in range(ncpu):
        pid = _clone(_CLONE_NEWPID | _SIGCHLD)
        if pid != 0:
            workers.append(pid)
        else:
            worker_index = i
            is_worker = True
            break

    if is_worker:
        for r, w in pipes:
            if w != pipes[worker_index][1]:
                os.close(w)
            os.close(r)

        start = time.time_ns()
        for _ in range(per_cpu_procs):
            pid = _clone(_CLONE_NEWPID | _SIGCHLD)
            if pid != 0:
                continue
            exit(0)
        end = time.time_ns()

        throughput = per_cpu_procs / ((end - start) / 1e9)

        write_fd = pipes[worker_index][1]
        os.write(write_fd, f"{throughput}\n".encode())
        os.close(write_fd)
        exit(0)

    else:
        for r, w in pipes:
            os.close(w)

        throughputs = []
        for i, (r, _) in enumerate(pipes):
            data = b""
            while True:
                chunk = os.read(r, 64)
                if not chunk:
                    break
                data += chunk
            os.close(r)
            throughputs.append(float(data.strip()))

        for pid in workers:
            os.waitpid(pid, 0)

        avg_throughput = sum(throughputs)
        total_procs = ncpu * per_cpu_procs
        print(f"Created {total_procs} processes across {ncpu} CPUs")
        # print(f"Per-worker throughputs: {[f'{t:.2f}' for t in throughputs]}")
        print(f"Average throughput: {avg_throughput:.2f} procs/second")
