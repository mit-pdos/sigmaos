#include <cstdlib>
#include <unordered_map>

#include <python/clntlib.h>

#include <proc/status.h>
#include <proxy/sigmap/sigmap.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

std::unique_ptr<sigmaos::proxy::sigmap::Clnt> clnt;
// Store all pointers
std::unordered_map<uintptr_t, CTstatProto*> stat_store;
std::unordered_map<uintptr_t, char*> string_store;

void init_socket() {
  clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();
}

// ======== Helper Functions ========

CTstatProto* convert_cstatproto(const TstatProto& src) {
  CTstatProto* dst = new CTstatProto;

  dst->type = src.type();
  dst->dev = src.dev();
  dst->qid.path = src.qid().path();
  dst->qid.version = src.qid().version();
  dst->qid.type = src.qid().type();
  dst->mode = src.mode();
  dst->atime = src.atime();
  dst->mtime = src.mtime();
  dst->length = src.length();

  // Allocate and copy strings
  dst->name = strdup(src.name().c_str());
  dst->uid = strdup(src.uid().c_str());
  dst->gid = strdup(src.gid().c_str());
  dst->muid = strdup(src.muid().c_str());

  return dst;
}

char* convert_cstr(std::shared_ptr<std::string> s) {
  return strdup(s->c_str());
}

// ============== Stubs =============

void close_fd_stub(int fd)
{
  clnt->CloseFD(fd);
}

CTstatProto* stat_stub(char* pn) {
  std::string pathname = pn;
  auto result = clnt->Stat(pathname);

  if (result.has_value()) {
    std::shared_ptr<TstatProto> sptr = result.value();
    TstatProto* raw_ptr = sptr.get();

    CTstatProto* py_res = convert_cstatproto(*raw_ptr);

    uintptr_t key = reinterpret_cast<uintptr_t>(py_res);
    stat_store[key] = py_res;
    
    // Convert to C definition
    return py_res;
  }

  log(sigmaos::proxy::sigmap::SPPROXYCLNT, "Stat failed");
  return NULL;
}

int create_stub(char* pn, uint32_t perm, uint32_t mode) {
  std::string pathname = pn;
  sigmaos::sigmap::types::Tperm p = perm;
  sigmaos::sigmap::types::Tmode m = mode;
  auto result_fd = clnt->Create(pathname, p, m);
  if (result_fd.has_value()) {
    return result_fd.value();
  }

  return -1; 
}

int open_stub(char *pn, uint32_t mode, bool wait) {
  std::string pathname = pn;
  sigmaos::sigmap::types::Tmode m = mode;
  auto result_fd = clnt->Open(pathname, m, wait);  
  if (result_fd.has_value()) {
    return result_fd.value();
  }

  return -1;
}

void rename_stub(char* src, char* dst) {
  std::string s = src;
  std::string d = dst;
  clnt->Rename(s, d);
}

void remove_stub(char* pn) {
  std::string pathname = pn;
  clnt->Remove(pathname);
}

char* get_file_stub(char* pn) {
  std::string pathname = pn;
  auto result = clnt->GetFile(pathname);
  if (result.has_value()) {
    char* py_res = convert_cstr(result.value());

    uintptr_t key = reinterpret_cast<uintptr_t>(py_res);
    string_store[key] = py_res;

    return py_res;
  }

  return NULL;
}

uint32_t put_file_stub(char* pn, uint32_t perm, uint32_t mode, char* data, uint64_t o, uint64_t l) 
{
  std::string pathname = pn;
  sigmaos::sigmap::types::Tperm p = perm;
  sigmaos::sigmap::types::Tmode m = mode;
  std::string d = data;
  sigmaos::sigmap::types::Toffset offset = o;
  sigmaos::sigmap::types::TleaseID leaseID = l;
  auto result = clnt->PutFile(pathname, p, m, &d, offset, leaseID);
  if (result.has_value()) {
    return result.value();
  }

  return -1;
}

uint32_t read_stub(int fd, char* b) {
  std::string bytes = b;
  auto result = clnt->Read(fd, &bytes);
  if (result.has_value()) {
    std::memcpy(b, bytes.c_str(), bytes.size() + 1);
    return result.value();
  }

  return -1;
}

uint32_t pread_stub(int fd, char* b, uint64_t o) {
  std::string bytes = b;
  sigmaos::sigmap::types::Toffset offset = o;
  auto result = clnt->Pread(fd, &bytes, offset);
  if (result.has_value()) {
    std::memcpy(b, bytes.c_str(), bytes.size() + 1);
    return result.value();
  }

  return -1;
}

uint32_t write_stub(int fd, char* b) {
  std::string bytes = b;
  auto result = clnt->Write(fd, &bytes);
  if (result.has_value()) {
    return result.value();
  }

  return -1;
}

void seek_stub(int fd, uint64_t o) {
  sigmaos::sigmap::types::Toffset offset = o;
  clnt->Seek(fd, offset);
}

// ========== ProcClnt API ==========

void started() 
{
  clnt->Started();
}

void exited(uint32_t status, char* msg)
{
  for (auto& [_, ptr] : stat_store) {
    free((void *) ptr->name);
    free((void *) ptr->uid);
    free((void *) ptr->gid);
    free((void *) ptr->muid);
    delete ptr;
  }
  stat_store.clear();

  for (auto& [_, ptr] : string_store) {
    free((void *) ptr);
  }
  string_store.clear();

  sigmaos::proc::Tstatus s = static_cast<sigmaos::proc::Tstatus>(status);
  std::string m = msg;
  clnt->Exited(s, m);
}

void wait_evict()
{
  clnt->WaitEvict();
}
