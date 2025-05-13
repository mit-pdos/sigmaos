import splib
import time

ms_start = time.time() * 1000.0
splib.Started()
ms_end = time.time() * 1000.0

ms_start_stat = time.time() * 1000.0
stat_res = splib.Stat("name/s3/~any/ivywu/dummy_package.py")
ms_end_stat = time.time() * 1000.0

print("Started:", ms_end - ms_start, "ms")
print("Stat:   ", ms_end_stat - ms_start_stat, "ms")

ms_start_exit = time.time() * 1000.0
splib.Exited(1, "Exited normally!")
ms_end_exit = time.time() * 1000.0
print("Exited: ", ms_end_exit - ms_start_exit, "ms")
