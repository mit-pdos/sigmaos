import os
import time

def _format_time_us(us):
    sec = us // 1_000_000
    us = us % 1_000_000

    t = time.localtime(sec)
    return (
        f"{t.tm_hour:02d}:"
        f"{t.tm_min:02d}:"
        f"{t.tm_sec:02d}."
        f"{us:06d}"
    )


def log_spawn_latency(msg: str, *, pid: str = None, spawn_time: int = None, op_start: int = None, now_ns: int = None):
    if now_ns is None:
        now_ns = time.time_ns()
    if pid is None:
        pid = os.environ.get("SIGMADEBUGPID", "")
    if spawn_time is None:
        spawn_time = int(os.environ.get("SIGMA_SPAWN_TIME", "0"))

    now_us = now_ns // 1_000

    since_spawn = 0
    if spawn_time is not None and spawn_time > 0:
        since_spawn = now_us - spawn_time
    since_op_start = 0
    if op_start is not None:
        since_op_start = now_us - (op_start // 1_000)

    ts = _format_time_us(now_us)
    print(f"{ts} {pid} SPAWN_LAT {msg} op:{since_op_start}us sinceSpawn:{since_spawn}us", flush=True)
