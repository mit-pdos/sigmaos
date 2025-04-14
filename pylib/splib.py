import ctypes

so_file = "/tmp/python/clntlib.so"
lib_funcs = ctypes.CDLL(so_file)
lib_funcs.write_raw_buffer.argtypes = [ctypes.c_char_p]
lib_funcs.write_seqno.argtypes = [ctypes.c_uint64]
lib_funcs.write_frame.argtypes = [ctypes.c_char_p, ctypes.c_uint32]
lib_funcs.write_frames.argtypes = [ctypes.POINTER(ctypes.c_char_p), ctypes.c_uint32, ctypes.POINTER(ctypes.c_uint64)]

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

def writeRawBuffer(buf):
    buffer = buf.encode("utf-8")
    lib_funcs.write_raw_buffer(buffer)

def writeSeqno(seqno):
    lib_funcs.write_seqno(seqno)

def writeFrame(frame):
    frameLen = len(frame)
    frameBuf = frame.encode("utf-8")
    lib_funcs.write_frame(frameBuf, frameLen)

def writeFrames(frames):
    numFrames = len(frames)
    frameLens = [len(f) for f in frames]
    frameLenArr = (ctypes.c_uint64 * numFrames)(*frameLens)
    encodedFrames = [f.encode("utf-8") for f in frames]
    frameArr = (ctypes.c_char_p * numFrames)(*encodedFrames)
    lib_funcs.write_frames(frameArr, ctypes.c_uint32(numFrames), frameLenArr)

def writeCall(seqno, frames):
    writeSeqno(seqno)
    writeFrames(frames)
