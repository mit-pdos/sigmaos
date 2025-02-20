import ctypes

so_file = "/tmp/python/clntlib.so"
lib_funcs = ctypes.CDLL(so_file)

def started():
    lib_funcs.init_socket()
    lib_funcs.started()

def exited():
    lib_funcs.exited()
