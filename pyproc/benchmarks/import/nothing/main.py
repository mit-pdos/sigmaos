import time
t = time.time_ns()

import splib
from splib.utils import log_spawn_latency
log_spawn_latency("Python ImportModules", op_start=t, now_ns=t)

splib.started()
splib.exited(splib.Status.Ok, "Exited normally!")
