import ctypes

so_file = "/tmp/python/clntlib.so"
lib_funcs = ctypes.CDLL(so_file)
lib_funcs.stat_stub.argtypes = [ctypes.c_char_p]

####################
#     Proc API     #
####################

def Started():
    lib_funcs.init_socket()
    lib_funcs.started()

def Exited():
    lib_funcs.exited()

###################
#     SPProxy     #
###################

def Stat(path):
    encoded_path = path.encode("utf-8")
    lib_funcs.stat_stub(encoded_path)
