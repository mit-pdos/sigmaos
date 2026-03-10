#!/usr/bin/env python3

import os


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


import time
import torch
import torch.nn as nn
import splib


def maybe_hold() -> None:
    hold_s = os.environ.get("ZYGOTE_BENCH_HOLD_SECS")
    if hold_s:
        time.sleep(float(hold_s))


MODEL = nn.Sequential(
    nn.Linear(512, 256),
    nn.ReLU(),
    nn.Linear(256, 64),
    nn.ReLU(),
    nn.Linear(64, 8),
)
MODEL.eval()
INPUT = torch.randn(1, 512)


if is_forking:
    _ = fork_point()

splib.started()
with torch.no_grad():
    out = MODEL(INPUT)
print(float(out.sum().item()))
maybe_hold()
splib.exited(splib.Status.Ok, "ok")
os._exit(0)
