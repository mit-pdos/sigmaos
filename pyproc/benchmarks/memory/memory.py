import os


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


import sys
import splib
import time
import random


def maybe_hold() -> None:
    hold_s = os.environ.get("ZYGOTE_BENCH_HOLD_SECS")
    if hold_s:
        time.sleep(float(hold_s))


# Allocate 100 MB of memory BEFORE forking
memory = bytearray(100 * 1024 * 1024)

if is_forking:
    args = fork_point()

splib.started()

# Modify some of the memory to trigger copy-on-write
memory[random.randint(0, len(memory) - 1)] = 1

maybe_hold()
splib.exited(splib.Status.Ok, "Ok")
os._exit(0)
