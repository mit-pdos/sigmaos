import ctypes
import concurrent.futures as _cf
import os

_lib = ctypes.CDLL("/usr/local/lib/libsigmaos_py.so")
_executor = _cf.ThreadPoolExecutor()

# sigmaos_new_clnt / sigmaos_new_clnt_tcp / sigmaos_free_clnt
_lib.sigmaos_new_clnt.restype = ctypes.c_void_p
_lib.sigmaos_new_clnt.argtypes = []

_lib.sigmaos_new_clnt_tcp.restype = ctypes.c_void_p
_lib.sigmaos_new_clnt_tcp.argtypes = [ctypes.c_char_p, ctypes.c_int]

_lib.sigmaos_free_clnt.restype = None
_lib.sigmaos_free_clnt.argtypes = [ctypes.c_void_p]

# sigmaos_started
_lib.sigmaos_started.restype = ctypes.c_int
_lib.sigmaos_started.argtypes = [ctypes.c_void_p]

# sigmaos_get_run_co_sandbox
_lib.sigmaos_get_run_co_sandbox.restype = ctypes.c_int
_lib.sigmaos_get_run_co_sandbox.argtypes = [ctypes.c_void_p]

# sigmaos_exited
_lib.sigmaos_exited.restype = ctypes.c_int
_lib.sigmaos_exited.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_char_p]

# sigmaos_get_file
_lib.sigmaos_get_file.restype = ctypes.c_void_p
_lib.sigmaos_get_file.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                   ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_free_buf
_lib.sigmaos_free_buf.restype = None
_lib.sigmaos_free_buf.argtypes = [ctypes.c_void_p]

# sigmaos_put_file
_lib.sigmaos_put_file.restype = ctypes.c_int
_lib.sigmaos_put_file.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                   ctypes.c_uint, ctypes.c_uint,
                                   ctypes.c_char_p, ctypes.c_size_t]

# sigmaos_s3_get_object
_lib.sigmaos_s3_get_object.restype = ctypes.c_void_p
_lib.sigmaos_s3_get_object.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                        ctypes.c_char_p, ctypes.c_int,
                                        ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_s3_put_object
_lib.sigmaos_s3_put_object.restype = ctypes.c_int
_lib.sigmaos_s3_put_object.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                        ctypes.c_char_p, ctypes.c_char_p,
                                        ctypes.c_size_t]

# sigmaos_s3_delegated_get_object
_lib.sigmaos_s3_delegated_get_object.restype = ctypes.c_void_p
_lib.sigmaos_s3_delegated_get_object.argtypes = [ctypes.c_void_p, ctypes.c_uint64,
                                                  ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_s3_delegated_get_object_view
_lib.sigmaos_s3_delegated_get_object_view.restype = ctypes.c_void_p
_lib.sigmaos_s3_delegated_get_object_view.argtypes = [ctypes.c_void_p, ctypes.c_uint64,
                                                       ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_ux_get_file
_lib.sigmaos_ux_get_file.restype = ctypes.c_void_p
_lib.sigmaos_ux_get_file.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                      ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_ux_put_file
_lib.sigmaos_ux_put_file.restype = ctypes.c_int
_lib.sigmaos_ux_put_file.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                      ctypes.c_char_p, ctypes.c_size_t]

# sigmaos_ux_delegated_get_file
_lib.sigmaos_ux_delegated_get_file.restype = ctypes.c_void_p
_lib.sigmaos_ux_delegated_get_file.argtypes = [ctypes.c_void_p, ctypes.c_uint64,
                                                ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_ux_delegated_get_file_view
_lib.sigmaos_ux_delegated_get_file_view.restype = ctypes.c_void_p
_lib.sigmaos_ux_delegated_get_file_view.argtypes = [ctypes.c_void_p, ctypes.c_uint64,
                                                     ctypes.POINTER(ctypes.c_size_t)]

# sigmaos_log_spawn_latency
_lib.sigmaos_log_spawn_latency.restype = None
_lib.sigmaos_log_spawn_latency.argtypes = [ctypes.c_void_p, ctypes.c_char_p,
                                            ctypes.c_uint64]

# sigmaos_get_use_shmem
_lib.sigmaos_get_use_shmem.restype = ctypes.c_int
_lib.sigmaos_get_use_shmem.argtypes = [ctypes.c_void_p]

# sigmaos_set_use_shmem
_lib.sigmaos_set_use_shmem.restype = None
_lib.sigmaos_set_use_shmem.argtypes = [ctypes.c_void_p, ctypes.c_int]

# sigmaos_last_error
_lib.sigmaos_last_error.restype = ctypes.c_char_p
_lib.sigmaos_last_error.argtypes = []

STATUS_OK      = 1
STATUS_EVICTED = 2
STATUS_ERR     = 3
STATUS_FATAL   = 4


def _last_error():
    return _lib.sigmaos_last_error().decode("utf-8")


class SigmaosClnt:
    def __init__(self, tcp_host: str = None, tcp_port: int = None):
        if tcp_host is not None and tcp_port is not None:
            self._clnt = _lib.sigmaos_new_clnt_tcp(
                tcp_host.encode("utf-8"), tcp_port)
            if not self._clnt:
                raise RuntimeError(
                    f"sigmaos_new_clnt_tcp failed: {_last_error()}")
        else:
            self._clnt = _lib.sigmaos_new_clnt()
            if not self._clnt:
                raise RuntimeError(f"sigmaos_new_clnt failed: {_last_error()}")

    def __del__(self):
        if self._clnt:
            _lib.sigmaos_free_clnt(self._clnt)
            self._clnt = None

    def get_run_co_sandbox(self) -> bool:
        return bool(_lib.sigmaos_get_run_co_sandbox(self._clnt))

    def get_use_shmem(self) -> bool:
        return bool(_lib.sigmaos_get_use_shmem(self._clnt))

    def started(self):
        rc = _lib.sigmaos_started(self._clnt)
        if rc != 0:
            raise RuntimeError(f"sigmaos_started failed: {_last_error()}")

    def exited(self, status=STATUS_OK, msg=""):
        rc = _lib.sigmaos_exited(self._clnt, status, msg.encode("utf-8"))
        if rc != 0:
            raise RuntimeError(f"sigmaos_exited failed: {_last_error()}")

    def get_file(self, pn: str, async_: bool = False):
        if async_:
            return _executor.submit(self.get_file, pn)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_get_file(self._clnt, pn.encode("utf-8"),
                                    ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_get_file({pn!r}) failed: {_last_error()}")
        try:
            return bytes(ctypes.cast(ptr, ctypes.POINTER(ctypes.c_char * out_len.value)).contents)
        finally:
            _lib.sigmaos_free_buf(ptr)

    def s3_get_object(self, bucket: str, key: str, cache: bool = False,
                      async_: bool = False):
        if async_:
            return _executor.submit(self.s3_get_object, bucket, key, cache)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_s3_get_object(self._clnt, bucket.encode("utf-8"),
                                          key.encode("utf-8"), int(cache),
                                          ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_s3_get_object({bucket!r}, {key!r}) failed: {_last_error()}")
        try:
            return bytes(ctypes.cast(ptr, ctypes.POINTER(ctypes.c_char * out_len.value)).contents)
        finally:
            _lib.sigmaos_free_buf(ptr)

    def s3_delegated_get_object_view(self, rpc_idx: int, async_: bool = False):
        if async_:
            return _executor.submit(self.s3_delegated_get_object_view, rpc_idx)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_s3_delegated_get_object_view(self._clnt, rpc_idx,
                                                         ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_s3_delegated_get_object_view({rpc_idx}) failed: {_last_error()}")
        arr = (ctypes.c_char * out_len.value).from_address(ptr)
        return memoryview(arr)

    def s3_delegated_get_object(self, rpc_idx: int, async_: bool = False):
        if async_:
            return _executor.submit(self.s3_delegated_get_object, rpc_idx)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_s3_delegated_get_object(self._clnt, rpc_idx,
                                                    ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_s3_delegated_get_object({rpc_idx}) failed: {_last_error()}")
        try:
            return bytes(ctypes.cast(ptr, ctypes.POINTER(ctypes.c_char * out_len.value)).contents)
        finally:
            _lib.sigmaos_free_buf(ptr)

    def s3_put_object(self, bucket: str, key: str, data: bytes) -> None:
        rc = _lib.sigmaos_s3_put_object(self._clnt, bucket.encode("utf-8"),
                                         key.encode("utf-8"), data, len(data))
        if rc != 0:
            raise RuntimeError(f"sigmaos_s3_put_object({bucket!r}, {key!r}) failed: {_last_error()}")

    def ux_get_file(self, path: str, async_: bool = False):
        if async_:
            return _executor.submit(self.ux_get_file, path)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_ux_get_file(self._clnt, path.encode("utf-8"),
                                        ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_ux_get_file({path!r}) failed: {_last_error()}")
        try:
            return bytes(ctypes.cast(ptr, ctypes.POINTER(ctypes.c_char * out_len.value)).contents)
        finally:
            _lib.sigmaos_free_buf(ptr)

    def ux_delegated_get_file_view(self, rpc_idx: int, async_: bool = False):
        if async_:
            return _executor.submit(self.ux_delegated_get_file_view, rpc_idx)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_ux_delegated_get_file_view(self._clnt, rpc_idx,
                                                       ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_ux_delegated_get_file_view({rpc_idx}) failed: {_last_error()}")
        arr = (ctypes.c_char * out_len.value).from_address(ptr)
        return memoryview(arr)

    def ux_delegated_get_file(self, rpc_idx: int, async_: bool = False):
        if async_:
            return _executor.submit(self.ux_delegated_get_file, rpc_idx)
        out_len = ctypes.c_size_t(0)
        ptr = _lib.sigmaos_ux_delegated_get_file(self._clnt, rpc_idx,
                                                  ctypes.byref(out_len))
        if not ptr:
            raise RuntimeError(f"sigmaos_ux_delegated_get_file({rpc_idx}) failed: {_last_error()}")
        try:
            return bytes(ctypes.cast(ptr, ctypes.POINTER(ctypes.c_char * out_len.value)).contents)
        finally:
            _lib.sigmaos_free_buf(ptr)

    def ux_put_file(self, path: str, data: bytes) -> None:
        rc = _lib.sigmaos_ux_put_file(self._clnt, path.encode("utf-8"),
                                       data, len(data))
        if rc != 0:
            raise RuntimeError(f"sigmaos_ux_put_file({path!r}) failed: {_last_error()}")

    def set_use_shmem(self, enable: bool) -> None:
        _lib.sigmaos_set_use_shmem(self._clnt, int(enable))

    def log_spawn_latency(self, label: str, elapsed_micros: int) -> None:
        _lib.sigmaos_log_spawn_latency(self._clnt, label.encode("utf-8"),
                                       elapsed_micros)

    def put_file(self, pn: str, data: bytes,
                 perm: int = 0o777, mode: int = 0) -> int:
        rc = _lib.sigmaos_put_file(self._clnt, pn.encode("utf-8"),
                                   perm, mode,
                                   data, len(data))
        if rc < 0:
            raise RuntimeError(f"sigmaos_put_file({pn!r}) failed: {_last_error()}")
        return rc
