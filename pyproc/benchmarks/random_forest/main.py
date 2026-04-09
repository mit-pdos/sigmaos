#!/usr/bin/env python3

import os

is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None

if is_forking:
    from splib.fork import fork_point

import time
import numpy as np
import joblib
import sklearn.ensemble  # ensure forest predict code is paged in

import splib

# Model is loaded before the fork point so all children share
# the in-memory representation via copy-on-write.
_MODEL_PATH = os.path.join(os.path.dirname(__file__), "model.joblib")
model = joblib.load(_MODEL_PATH)

def maybe_hold() -> None:
    hold_s = os.environ.get("ZYGOTE_BENCH_HOLD_SECS")
    if hold_s:
        time.sleep(float(hold_s))

if is_forking:
    _ = fork_point()

splib.started()

rng = np.random.default_rng(seed=0xDEADBEEF)
X = rng.standard_normal((10_000, 20))
preds = model.predict(X)
result = int(preds.sum())
print(result)

maybe_hold()
splib.exited(splib.Status.Ok, "ok")
os._exit(0)
