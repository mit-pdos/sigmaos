#!/usr/bin/env python3

import os


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


import time
import numpy as np
import pandas as pd
import splib


def maybe_hold() -> None:
    hold_s = os.environ.get("ZYGOTE_BENCH_HOLD_SECS")
    if hold_s:
        time.sleep(float(hold_s))


if is_forking:
    _ = fork_point()

splib.started()
arr = np.arange(20000, dtype=np.float64).reshape(200, 100)
frame = pd.DataFrame(arr)
out = frame.mean(axis=0).sum()
print(float(out))
maybe_hold()
splib.exited(splib.Status.Ok, "ok")
os._exit(0)
