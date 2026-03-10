#!/usr/bin/env python3

import os


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


import splib
import time


def maybe_hold() -> None:
    hold_s = os.environ.get("ZYGOTE_BENCH_HOLD_SECS")
    if hold_s:
        time.sleep(float(hold_s))


if is_forking:
    _ = fork_point()

splib.started()
print("hello from zygote benchmark")
maybe_hold()
splib.exited(splib.Status.Ok, "ok")
os._exit(0)
