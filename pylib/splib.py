import ctypes

so_file = "/tmp/python/clntlib.so"
lib_funcs = ctypes.CDLL(so_file)
lib_funcs.write_seqno.argtypes = [ctypes.c_uint64]
lib_funcs.write_raw_buffer.argtypes = [ctypes.c_char_p]

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

def writeSeqno(seqno):
    lib_funcs.write_seqno(seqno)

def writeRawBuffer(buf):
    buffer = buf.encode("utf-8")
    lib_funcs.write_raw_buffer(buffer)
