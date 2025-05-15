import ctypes

so_file = "/tmp/python/clntlib.so"
lib_funcs = ctypes.CDLL(so_file)

class CTqidProto(ctypes.Structure):
    _fields_ = [
        ('type', ctypes.c_uint32),
        ('version', ctypes.c_uint32),
        ('path', ctypes.c_uint64),
    ]

class CTstatProto(ctypes.Structure):
    _fields_ = [
        ('type', ctypes.c_uint32),
        ('dev', ctypes.c_uint32),
        ('qid', CTqidProto),
        ('mode', ctypes.c_uint32),
        ('atime', ctypes.c_uint32),
        ('mtime', ctypes.c_uint32),
        ('length', ctypes.c_uint64),
        ('name', ctypes.c_char_p),
        ('uid', ctypes.c_char_p),
        ('gid', ctypes.c_char_p),
        ('muid', ctypes.c_char_p),
    ]

####################
#     Proc API     #
####################

def Started():
    lib_funcs.init_socket()
    lib_funcs.started()

lib_funcs.exited.argtypes = [ctypes.c_uint32, ctypes.c_char_p]
def Exited(status, message):
    lib_funcs.exited(ctypes.c_uint32(status), ctypes.c_char_p(message.encode("utf-8")))

###################
#     SPProxy     #
###################

lib_funcs.close_fd_stub.argtypes = [ctypes.c_int]
def CloseFD(fd):
    lib_funcs.close_fd_stub(ctypes.c_int(fd))

lib_funcs.stat_stub.argtypes = [ctypes.c_char_p]
lib_funcs.stat_stub.restype = ctypes.POINTER(CTstatProto)
def Stat(pathname):
    return lib_funcs.stat_stub(ctypes.c_char_p(pathname.encode("utf-8")))

lib_funcs.create_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_uint32]
lib_funcs.create_stub.restype = ctypes.c_int
def Create(pathname, perm, mode):
    return lib_funcs.create_stub(ctypes.c_char_p(pathname.encode("utf-8")), ctypes.c_uint32(perm), ctypes.c_uint32(mode))

lib_funcs.open_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_bool]
lib_funcs.open_stub.restype = ctypes.c_int
def Open(pathname, mode, wait):
    return lib_funcs.open_stub(ctypes.c_char_p(pathname.encode("utf-8")), ctypes.c_uint32(mode), ctypes.c_bool(wait))

lib_funcs.rename_stub.argtypes = [ctypes.c_char_p, ctypes.c_char_p]
def Rename(src, dst):
    lib_funcs.rename_stub(ctypes.c_char_p(src.encode("utf-8")), ctypes.c_char_p(dst.encode("utf-8")))

lib_funcs.remove_stub.argtypes = [ctypes.c_char_p]
def Remove(pathname):
    lib_funcs.remove_stub(ctypes.c_char_p(pathname.encode("utf-8")))

lib_funcs.get_file_stub.argtypes = [ctypes.c_char_p]
lib_funcs.get_file_stub.restype = ctypes.c_char_p 
def GetFile(pathname):
    return lib_funcs.get_file_stub(ctypes.c_char_p(pathname.encode("utf-8")))

lib_funcs.put_file_stub.argtypes = [ctypes.c_char_p, ctypes.c_uint32, ctypes.c_uint32, ctypes.c_char_p, ctypes.c_uint64, ctypes.c_uint64]
lib_funcs.put_file_stub.restype = ctypes.c_uint32
def PutFile(pathname, perm, mode, data, offset, leaseID):
    return lib_funcs.put_file_stub(ctypes.c_char_p(pathname.encode("utf-8")), ctypes.c_uint32(perm), ctypes.c_uint32(mode), ctypes.c_char_p(data.encode("utf-8")), ctypes.c_uint64(offset), ctypes.c_uint64(leaseID))

lib_funcs.read_stub.argtypes = [ctypes.c_int, ctypes.c_char_p]
lib_funcs.read_stub.restype = ctypes.c_uint32
def Read(fd, b):
    return lib_funcs.read_stub(ctypes.c_int(fd), b) 

lib_funcs.pread_stub.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_uint64]
lib_funcs.pread_stub.restype = ctypes.c_uint32
def Pread(fd, b, offset):
    return lib_funcs.pread_stub(ctypes.c_int(fd), ctypes.c_char_p(b.encode("utf-8")), ctypes.c_uint64(offset))

lib_funcs.write_stub.argtypes = [ctypes.c_int, ctypes.c_char_p]
lib_funcs.write_stub.restype = ctypes.c_uint32
def Write(fd, b):
    return lib_funcs.write_stub(ctypes.c_int(fd), ctypes.c_char_p(b.encode("utf-8")))

lib_funcs.seek_stub.argtypes = [ctypes.c_int, ctypes.c_uint64]
def Seek(fd, offset):
    lib_funcs.seek_stub(ctypes.c_int(fd), ctypes.c_uint64(offset))

lib_funcs.clnt_id_stub.restype = ctypes.c_uint64
def ClntID():
    return lib_funcs.clnt_id_stub()
