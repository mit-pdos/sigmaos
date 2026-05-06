#pragma once

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Status constants matching sigmaos::proc::Tstatus
#define SIGMAOS_STATUS_OK 1
#define SIGMAOS_STATUS_EVICTED 2
#define SIGMAOS_STATUS_ERR 3
#define SIGMAOS_STATUS_FATAL 4

typedef void* SigmaosClnt;

SigmaosClnt sigmaos_new_clnt();
SigmaosClnt sigmaos_new_clnt_tcp(const char* host, int port);
void sigmaos_free_clnt(SigmaosClnt clnt);

// Returns 0 on success, -1 on error.
int sigmaos_started(SigmaosClnt clnt);

// Returns 1 if the proc env has RunCoSandbox set, 0 otherwise.
int sigmaos_get_run_co_sandbox(SigmaosClnt clnt);

// status should be one of the SIGMAOS_STATUS_* constants.
// Returns 0 on success, -1 on error.
int sigmaos_exited(SigmaosClnt clnt, int status, const char* msg);

// Returns malloc'd buffer of *out_len bytes, or NULL on error.
// Caller must free with sigmaos_free_buf().
char* sigmaos_get_file(SigmaosClnt clnt, const char* pn, size_t* out_len);
void sigmaos_free_buf(char* buf);

// Returns bytes written on success, -1 on error.
int sigmaos_put_file(SigmaosClnt clnt, const char* pn, unsigned int perm,
                     unsigned int mode, const char* data, size_t len);

// Returns malloc'd buffer of *out_len bytes, or NULL on error.
// Caller must free with sigmaos_free_buf().
char* sigmaos_s3_get_object(SigmaosClnt clnt, const char* bucket,
                            const char* key, int cache, size_t* out_len);

// Returns 0 on success, -1 on error.
int sigmaos_s3_put_object(SigmaosClnt clnt, const char* bucket, const char* key,
                          const char* data, size_t len);

// Returns malloc'd buffer of *out_len bytes, or NULL on error.
// Caller must free with sigmaos_free_buf().
char* sigmaos_s3_delegated_get_object(SigmaosClnt clnt, uint64_t rpc_idx,
                                      size_t* out_len);

// Returns pointer into the delegated reply buffer (no malloc, no free needed),
// or NULL on error. Valid for the proc's lifetime. The client retains ownership
// of the underlying DelegatedBuf; callers must not free the returned pointer.
const char* sigmaos_s3_delegated_get_object_view(SigmaosClnt clnt,
                                                  uint64_t rpc_idx,
                                                  size_t* out_len);

// Returns malloc'd buffer of *out_len bytes, or NULL on error.
// Caller must free with sigmaos_free_buf().
char* sigmaos_ux_get_file(SigmaosClnt clnt, const char* path, size_t* out_len);

// Returns 0 on success, -1 on error.
int sigmaos_ux_put_file(SigmaosClnt clnt, const char* path, const char* data,
                        size_t len);

// Returns malloc'd buffer of *out_len bytes, or NULL on error.
// Caller must free with sigmaos_free_buf().
char* sigmaos_ux_delegated_get_file(SigmaosClnt clnt, uint64_t rpc_idx,
                                    size_t* out_len);

// Returns pointer into the delegated reply buffer (no malloc, no free needed),
// or NULL on error. Valid for the proc's lifetime. The client retains ownership
// of the underlying DelegatedBuf; callers must not free the returned pointer.
const char* sigmaos_ux_delegated_get_file_view(SigmaosClnt clnt,
                                               uint64_t rpc_idx,
                                               size_t* out_len);

// Logs a spawn-latency phase. label is the phase name (e.g.
// "Paper.Initialization.TransferState"); elapsed_micros is the duration of the
// phase measured by the proc using a monotonic clock.
void sigmaos_log_spawn_latency(SigmaosClnt clnt, const char* label,
                               uint64_t elapsed_micros);

// Returns a thread-local string describing the last error, or "" if none.
const char* sigmaos_last_error();

#ifdef __cplusplus
}
#endif
