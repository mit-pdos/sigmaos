import time

start_import_ns = time.time_ns()
import sklearn
end_import_ns = time.time_ns()

import splib
from splib.utils import log_spawn_latency
log_spawn_latency("Python ImportModules", op_start=start_import_ns, now_ns=end_import_ns)

splib.started()
splib.exited(splib.Status.Ok, "Exited normally!")
