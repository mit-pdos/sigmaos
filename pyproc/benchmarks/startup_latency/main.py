import time

startup_time_ns = time.time_ns()

import os
import splib
from splib.utils import log_spawn_latency


is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None
if is_forking:
    from splib.fork import fork_point


if is_forking:
    _ = fork_point()
    log_spawn_latency("E2e spawn time since fork until main")
else:
    log_spawn_latency("E2e spawn time since spawn until main", now_ns=startup_time_ns)

splib.started()
splib.exited(splib.Status.Ok, "ok")