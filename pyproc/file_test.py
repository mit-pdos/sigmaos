import splib

splib.Started()
# File creation
pathname = "name/my_file"
fd = splib.Create(pathname, 777, 0x01)
print("Fd:", fd)
# Write to file
data = "hello"
written = splib.Write(fd, data)
print("Written:", written)
# Get the file contents
contents = splib.GetFile(pathname)
print("Contents:", contents)
splib.Exited(1, "Exited normally!")
