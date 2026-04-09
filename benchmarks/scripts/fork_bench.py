from __future__ import annotations

import gc
gc.disable()

import ctypes
import ctypes.util
import os
import time
import signal
import argparse


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

    parser = argparse.ArgumentParser(description="Fork benchmark")
    parser.add_argument("-n", "--ncpu", type=int, default=os.cpu_count(), help="Number of workers")
    parser.add_argument("-r", "--rate", type=int, default=0, help="Total fork rate across all CPUs (procs/second)")
    parser.add_argument("-p", "--per-cpu-procs", type=int, default=1000, help="Number of forks per CPU")
    args = parser.parse_args()

    ncpu = args.ncpu
    rate = args.rate
    per_cpu_procs = args.per_cpu_procs
    per_cpu_rate = rate / ncpu
    per_cpu_interval = 1 / per_cpu_rate if per_cpu_rate > 0 else 0

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

        signal.signal(signal.SIGCHLD, signal.SIG_IGN)

        start = time.time_ns()
        next_time = time.perf_counter() + per_cpu_interval
        for _ in range(per_cpu_procs):
            pid = _clone(_CLONE_NEWPID | _SIGCHLD)
            if pid != 0:
                sleep_time = next_time - time.perf_counter()
                if sleep_time > 0:
                    time.sleep(sleep_time)
                next_time += per_cpu_interval
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
        print(f"Average throughput: {avg_throughput:.2f} procs/second")
