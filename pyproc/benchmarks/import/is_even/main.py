import time
from splib.utils import log_spawn_latency

start_import_ns = time.time_ns()
import is_even
log_spawn_latency("Python ImportModules", op_start=start_import_ns)

import splib
splib.started()
splib.exited(splib.Status.Ok, "Exited normally!")
