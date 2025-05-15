import splib
import time
import ctypes

results = []

# ms_start = time.time() * 1000.0
# splib.Open("name/msched/~local/", 2, False)
# ms_end = time.time() * 1000.0
# results.append(("GetDirLocal", ms_end - ms_start))

# ms_start = time.time() * 1000.0
# splib.Open("name/ux/~local/", 2, False)
# ms_end = time.time() * 1000.0
# results.append(("GetDirUx", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Started()
ms_end = time.time() * 1000.0
results.append(("Started", ms_end - ms_start))

# Local

ms_start = time.time() * 1000.0
splib.PutFile("name/ux/~local/pystat_file", 777, 0, "Hello World!", 0, 0)
ms_end = time.time() * 1000.0
results.append(("PutFile_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
file_contents = splib.GetFile("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("GetFile_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Stat("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Stat_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
fd = splib.Create("name/ux/~local/pystat_file_2", 777, 2)
ms_end = time.time() * 1000.0
results.append(("Create_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Write(fd, "Hello!")
ms_end = time.time() * 1000.0
results.append(("Write_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.CloseFD(fd)
ms_end = time.time() * 1000.0
results.append(("CloseFD_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
fd = splib.Open("name/ux/~local/pystat_file_2", 2, False)
ms_end = time.time() * 1000.0
results.append(("Open_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Seek(fd, 0)
ms_end = time.time() * 1000.0
results.append(("Seek_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
buf = ctypes.create_string_buffer(10)
splib.Read(fd, buf)
ms_end = time.time() * 1000.0
results.append(("Read_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.CloseFD(fd)
ms_end = time.time() * 1000.0
results.append(("CloseFD_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Remove("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Remove_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Remove("name/ux/~local/pystat_file_2")
ms_end = time.time() * 1000.0
results.append(("Remove_Local", ms_end - ms_start))

# Remote

ms_start = time.time() * 1000.0
splib.PutFile("name/s3/~local/ivywu/pystat_file", 777, 0, "Hello World!", 0, 0)
ms_end = time.time() * 1000.0
results.append(("PutFile_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
file_contents = splib.GetFile("name/s3/~local/ivywu/pystat_file")
ms_end = time.time() * 1000.0
results.append(("GetFile_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Stat("name/s3/~local/ivywu/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Stat_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Remove("name/s3/~local/ivywu/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Remove_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.Exited(1, "Exited normally!")
ms_end = time.time() * 1000.0
results.append(("Exited", ms_end - ms_start))

for r in results:
    print(r[0], r[1])
