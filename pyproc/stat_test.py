import splib

splib.Started()
stat_res = splib.Stat("name/s3/~any/ivywu/dummy_package.py")
if stat_res:
    print("Name:", stat_res.contents.name.decode())
    print("Type:", stat_res.contents.type)
    print("Length:", stat_res.contents.length)
splib.Exited(1, "Exited normally!")
