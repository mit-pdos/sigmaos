#include <proc/proc.h>
#include <proc/status.h>
#include <proxy/buf/buf.h>
#include <proxy/s3/clnt.h>
#include <proxy/sigmap/python/sigmaos_c.h>
#include <proxy/sigmap/sigmap.h>
#include <proxy/ux/clnt.h>
#include <util/perf/perf.h>

#include <chrono>
#include <cstring>
#include <map>
#include <memory>
#include <string>

struct SigmaosClntState {
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp;
  std::shared_ptr<sigmaos::proxy::s3::Clnt> s3;
  std::shared_ptr<sigmaos::proxy::ux::Clnt> ux;
  // Keeps DataBuf alive while Python holds a memoryview into its data.
  std::map<uint64_t, std::shared_ptr<sigmaos::proxy::buf::DataBuf>>
      data_bufs;
};

static SigmaosClntState* state(SigmaosClnt clnt) {
  return static_cast<SigmaosClntState*>(clnt);
}

// Thread-local error storage
static thread_local std::string tl_last_error;

static void set_error(const std::string& msg) { tl_last_error = msg; }

static void clear_error() { tl_last_error.clear(); }

static const std::string S3_SVC_BASE = "name/s3/";
static const std::string UX_SVC_BASE = "name/ux/";

static sigmaos::proxy::s3::Clnt* s3_clnt(SigmaosClntState* st) {
  if (!st->s3) {
    std::string kid = st->sp->ProcEnv()->GetKernelID();
    st->s3 =
        std::make_shared<sigmaos::proxy::s3::Clnt>(st->sp, S3_SVC_BASE + kid);
  }
  return st->s3.get();
}

static sigmaos::proxy::ux::Clnt* ux_clnt(SigmaosClntState* st) {
  if (!st->ux) {
    std::string kid = st->sp->ProcEnv()->GetKernelID();
    st->ux =
        std::make_shared<sigmaos::proxy::ux::Clnt>(st->sp, UX_SVC_BASE + kid);
  }
  return st->ux.get();
}

extern "C" {

SigmaosClnt sigmaos_new_clnt() {
  clear_error();
  try {
    auto* st = new SigmaosClntState();
    st->sp = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
    return st;
  } catch (const std::exception& e) {
    set_error(e.what());
    return nullptr;
  }
}

SigmaosClnt sigmaos_new_clnt_tcp(const char* host, int port) {
  clear_error();
  try {
    auto* st = new SigmaosClntState();
    st->sp = std::make_shared<sigmaos::proxy::sigmap::Clnt>(std::string(host),
                                                            port);
    return st;
  } catch (const std::exception& e) {
    set_error(e.what());
    return nullptr;
  }
}

void sigmaos_free_clnt(SigmaosClnt clnt) { delete state(clnt); }

int sigmaos_started(SigmaosClnt clnt) {
  clear_error();
  auto res = state(clnt)->sp->Started();
  if (!res.has_value()) {
    set_error(res.error().String());
    return -1;
  }
  return 0;
}

int sigmaos_get_run_co_sandbox(SigmaosClnt clnt) {
  return state(clnt)->sp->ProcEnv()->GetRunCoSandbox() ? 1 : 0;
}

int sigmaos_exited(SigmaosClnt clnt, int status, const char* msg) {
  clear_error();
  std::string msg_str(msg ? msg : "");
  auto tstatus = static_cast<sigmaos::proc::Tstatus>(status);
  auto res = state(clnt)->sp->Exited(tstatus, msg_str);
  if (!res.has_value()) {
    set_error(res.error().String());
    return -1;
  }
  return 0;
}

char* sigmaos_get_file(SigmaosClnt clnt, const char* pn, size_t* out_len) {
  clear_error();
  auto res = state(clnt)->sp->GetFile(std::string(pn));
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto& s = res.value();
  *out_len = s->size();
  char* buf = static_cast<char*>(malloc(s->size()));
  if (buf) {
    memcpy(buf, s->data(), s->size());
  }
  return buf;
}


void sigmaos_free_buf(char* buf) { free(buf); }

int sigmaos_put_file(SigmaosClnt clnt, const char* pn, unsigned int perm,
                     unsigned int mode, const char* data, size_t len) {
  clear_error();
  std::string data_str(data, len);
  auto res = state(clnt)->sp->PutFile(
      std::string(pn), static_cast<sigmaos::sigmap::types::Tperm>(perm),
      static_cast<sigmaos::sigmap::types::Tmode>(mode), &data_str, 0, 0);
  if (!res.has_value()) {
    set_error(res.error().String());
    return -1;
  }
  return static_cast<int>(res.value());
}

char* sigmaos_s3_get_object(SigmaosClnt clnt, const char* bucket,
                            const char* key, int cache, size_t* out_len) {
  clear_error();
  auto res = s3_clnt(state(clnt))
                 ->GetObject(std::string(bucket), std::string(key), cache != 0);
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto& s = res.value();
  *out_len = s->size();
  char* buf = static_cast<char*>(malloc(s->size()));
  if (buf) {
    memcpy(buf, s->data(), s->size());
  }
  return buf;
}

int sigmaos_s3_put_object(SigmaosClnt clnt, const char* bucket, const char* key,
                          const char* data, size_t len) {
  clear_error();
  std::string data_str(data, len);
  auto res = s3_clnt(state(clnt))
                 ->PutObject(std::string(bucket), std::string(key), &data_str);
  if (!res.has_value()) {
    set_error(res.error().String());
    return -1;
  }
  return 0;
}

char* sigmaos_s3_delegated_get_object(SigmaosClnt clnt, uint64_t rpc_idx,
                                      size_t* out_len) {
  clear_error();
  auto res = s3_clnt(state(clnt))->DelegatedGetObject(rpc_idx);
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto& dbuf = res.value();
  *out_len = dbuf->size();
  char* buf = static_cast<char*>(malloc(dbuf->size()));
  if (buf) {
    memcpy(buf, dbuf->data(), dbuf->size());
  }
  return buf;
}

const char* sigmaos_s3_delegated_get_object_view(SigmaosClnt clnt,
                                                  uint64_t rpc_idx,
                                                  size_t* out_len) {
  clear_error();
  auto res = s3_clnt(state(clnt))->DelegatedGetObject(rpc_idx);
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto dbuf = std::move(res.value());
  *out_len = dbuf->size();
  const char* ptr = dbuf->data();
  state(clnt)->data_bufs[rpc_idx] = std::move(dbuf);
  return ptr;
}

char* sigmaos_ux_get_file(SigmaosClnt clnt, const char* path, size_t* out_len) {
  clear_error();
  auto res = ux_clnt(state(clnt))->GetFile(std::string(path));
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto& s = res.value();
  *out_len = s->size();
  char* buf = static_cast<char*>(malloc(s->size()));
  if (buf) {
    memcpy(buf, s->data(), s->size());
  }
  return buf;
}

int sigmaos_ux_put_file(SigmaosClnt clnt, const char* path, const char* data,
                        size_t len) {
  clear_error();
  std::string data_str(data, len);
  auto res = ux_clnt(state(clnt))->PutFile(std::string(path), &data_str);
  if (!res.has_value()) {
    set_error(res.error().String());
    return -1;
  }
  return 0;
}

char* sigmaos_ux_delegated_get_file(SigmaosClnt clnt, uint64_t rpc_idx,
                                    size_t* out_len) {
  clear_error();
  auto res = ux_clnt(state(clnt))->DelegatedGetFile(rpc_idx);
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto& dbuf = res.value();
  *out_len = dbuf->size();
  char* buf = static_cast<char*>(malloc(dbuf->size()));
  if (buf) {
    memcpy(buf, dbuf->data(), dbuf->size());
  }
  return buf;
}

const char* sigmaos_ux_delegated_get_file_view(SigmaosClnt clnt,
                                               uint64_t rpc_idx,
                                               size_t* out_len) {
  clear_error();
  auto res = ux_clnt(state(clnt))->DelegatedGetFile(rpc_idx);
  if (!res.has_value()) {
    set_error(res.error().String());
    *out_len = 0;
    return nullptr;
  }
  auto dbuf = std::move(res.value());
  *out_len = dbuf->size();
  const char* ptr = dbuf->data();
  state(clnt)->data_bufs[rpc_idx] = std::move(dbuf);
  return ptr;
}

void sigmaos_log_spawn_latency(SigmaosClnt clnt, const char* label,
                               uint64_t elapsed_micros) {
  auto now_us = std::chrono::duration_cast<std::chrono::microseconds>(
                    std::chrono::high_resolution_clock::now().time_since_epoch())
                    .count();
  int64_t op_start_us = now_us - static_cast<int64_t>(elapsed_micros);
  google::protobuf::Timestamp op_start;
  op_start.set_seconds(op_start_us / 1000000);
  op_start.set_nanos(static_cast<int32_t>((op_start_us % 1000000) * 1000));
  auto* sp = state(clnt)->sp.get();
  LogSpawnLatency(sp->ProcEnv()->GetPID(), sp->ProcEnv()->GetSpawnTime(),
                  op_start, std::string(label));
}

int sigmaos_get_use_shmem(SigmaosClnt clnt) {
  return state(clnt)->sp->ProcEnv()->GetUseShmem() ? 1 : 0;
}

void sigmaos_set_use_shmem(SigmaosClnt clnt, int enable) {
  state(clnt)->sp->SetUseShmem(enable != 0);
}

void sigmaos_reset_proc_env() { sigmaos::proc::ResetProcEnv(); }

const char* sigmaos_last_error() { return tl_last_error.c_str(); }

}  // extern "C"
