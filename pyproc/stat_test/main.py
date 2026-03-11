import _clntlib as splib
import time

results = []
bucket_name = "ncam"

ms_start = time.time() * 1000.0
splib.init_socket()
ms_end = time.time() * 1000.0
results.append(("Init", ms_end - ms_start))

# ms_start = time.time() * 1000.0
# splib.Open("name/msched/~local/", 2, False)
# ms_end = time.time() * 1000.0
# results.append(("GetDirLocal", ms_end - ms_start))

# ms_start = time.time() * 1000.0
# splib.Open("name/ux/~local/", 2, False)
# ms_end = time.time() * 1000.0
# results.append(("GetDirUx", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.started()
ms_end = time.time() * 1000.0
results.append(("Started", ms_end - ms_start))

# Local

ms_start = time.time() * 1000.0
splib.clnt_id()
ms_end = time.time() * 1000.0
results.append(("ClntID", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.put_file("name/ux/~local/pystat_file", 777, 0, "Hello World!", 0, 0)
ms_end = time.time() * 1000.0
results.append(("PutFile_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
file_contents = splib.get_file("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("GetFile_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.stat("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Stat_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
fd = splib.create("name/ux/~local/pystat_file_2", 777, 2)
ms_end = time.time() * 1000.0
results.append(("Create_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.write(fd, "Hello!")
ms_end = time.time() * 1000.0
results.append(("Write_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.close(fd)
ms_end = time.time() * 1000.0
results.append(("CloseFD_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
fd = splib.open("name/ux/~local/pystat_file_2", 2, False)
ms_end = time.time() * 1000.0
results.append(("Open_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.seek(fd, 0)
ms_end = time.time() * 1000.0
results.append(("Seek_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.read(fd, 10)
ms_end = time.time() * 1000.0
results.append(("Read_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.close(fd)
ms_end = time.time() * 1000.0
results.append(("CloseFD_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.remove("name/ux/~local/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Remove_Local", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.remove("name/ux/~local/pystat_file_2")
ms_end = time.time() * 1000.0
results.append(("Remove_Local", ms_end - ms_start))

# Remote

ms_start = time.time() * 1000.0
splib.put_file(f"name/s3/~local/{bucket_name}/pystat_file", 777, 0, "Hello World!", 0, 0)
ms_end = time.time() * 1000.0
results.append(("PutFile_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
file_contents = splib.get_file(f"name/s3/~local/{bucket_name}/pystat_file")
ms_end = time.time() * 1000.0
results.append(("GetFile_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.stat(f"name/s3/~local/{bucket_name}/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Stat_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.remove(f"name/s3/~local/{bucket_name}/pystat_file")
ms_end = time.time() * 1000.0
results.append(("Remove_Remote", ms_end - ms_start))

ms_start = time.time() * 1000.0
splib.exited(splib.Status.Ok, "Exited normally!")
ms_end = time.time() * 1000.0
results.append(("Exited", ms_end - ms_start))

for r in results:
    print(r[0], r[1])
