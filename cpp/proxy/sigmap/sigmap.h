#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <util/perf/perf.h>
#include <io/conn/conn.h>
#include <io/conn/unix/unix.h>
#include <io/transport/transport.h>
#include <io/demux/clnt.h>
#include <serr/serr.h>
#include <rpc/clnt.h>
#include <proc/proc.h>
#include <proc/status.h>
#include <sigmap/types.h>
#include <sigmap/const.h>
#include <sigmap/named.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET_PN = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET
const std::string SPPROXYCLNT = "SPPROXYCLNT";
const std::string SPPROXYCLNT_ERR = "SPPROXYCLNT" + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt() : _disconnected(false) {
    _env = sigmaos::proc::GetProcEnv();
    log(SPPROXYCLNT, "New clnt {}", _env->String());
    auto start = google::protobuf::util::TimeUtil::GetCurrentTime();
    _conn = std::make_shared<sigmaos::io::conn::unixconn::ClntConn>(SPPROXY_SOCKET_PN);
    LogSpawnLatency(_env->GetPID(), _env->GetSpawnTime(), start, "Connect ClntConn");
    start = google::protobuf::util::TimeUtil::GetCurrentTime();
    _trans = std::make_shared<sigmaos::io::transport::Transport>(_conn);
    LogSpawnLatency(_env->GetPID(), _env->GetSpawnTime(), start, "Create transport");
    start = google::protobuf::util::TimeUtil::GetCurrentTime();
    _demux = std::make_shared<sigmaos::io::demux::Clnt>(_trans);
    LogSpawnLatency(_env->GetPID(), _env->GetSpawnTime(), start, "Create demuxclnt");
    start = google::protobuf::util::TimeUtil::GetCurrentTime();
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(_demux);
    LogSpawnLatency(_env->GetPID(), _env->GetSpawnTime(), start, "Create rpcclnt");
    start = google::protobuf::util::TimeUtil::GetCurrentTime();
    log(SPPROXYCLNT, "Initializing proxy conn");
    // Initialize the sigmaproxyd connection
    init_conn();
    LogSpawnLatency(_env->GetPID(), _env->GetSpawnTime(), start, "Init spproxy conn");
  }

  ~Clnt() { Close(); }
  void Close() { 
    log(SPPROXYCLNT, "Close");
    _rpcc->Close(); 
    log(SPPROXYCLNT, "Done close");
  }

  std::expected<int, sigmaos::serr::Error> Test();
  std::shared_ptr<sigmaos::proc::ProcEnv> ProcEnv() { return _env; }

  // Stubs

  std::expected<int, sigmaos::serr::Error> CloseFD(int fd);
  std::expected<std::shared_ptr<TstatProto>, sigmaos::serr::Error> Stat(std::string pn);
  std::expected<int, sigmaos::serr::Error> Create(std::string pn, sigmaos::sigmap::types::Tperm perm, sigmaos::sigmap::types::Tmode mode);
  std::expected<int, sigmaos::serr::Error> Open(std::string pn, sigmaos::sigmap::types::Tmode mode, bool wait);
  std::expected<int, sigmaos::serr::Error> Rename(std::string src, std::string dst);
  std::expected<int, sigmaos::serr::Error> Remove(std::string pn);
  std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> GetFile(std::string pn);
  std::expected<uint32_t, sigmaos::serr::Error> PutFile(std::string pn, sigmaos::sigmap::types::Tperm perm, sigmaos::sigmap::types::Tmode mode, std::string *data, sigmaos::sigmap::types::Toffset offset, sigmaos::sigmap::types::TleaseID leaseID);
  std::expected<uint32_t, sigmaos::serr::Error> Read(int fd, std::string *b);
  std::expected<uint32_t, sigmaos::serr::Error> Pread(int fd, std::string *b, sigmaos::sigmap::types::Toffset offset);
  std::expected<uint32_t, sigmaos::serr::Error> Write(int fd, std::string *b);
  std::expected<int, sigmaos::serr::Error> Seek(int fd, sigmaos::sigmap::types::Toffset offset);
  // TODO: fence type in CreateLeased?
  std::expected<int, sigmaos::serr::Error> CreateLeased(std::string path, sigmaos::sigmap::types::Tperm perm, sigmaos::sigmap::types::Tmode mode, sigmaos::sigmap::types::TleaseID leaseID/*, f sp.Tfence*/);
  std::expected<sigmaos::sigmap::types::TclntID, sigmaos::serr::Error> ClntID();
  // TODO: fence type in FenceDir?
  std::expected<int, sigmaos::serr::Error> FenceDir(std::string pn/*, f sp.Tfence*/);
  // TODO: support WriteFence?
  //func (scc *SPProxyClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
  std::expected<int, sigmaos::serr::Error> WriteRead(int fd, std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov);
  std::expected<int, sigmaos::serr::Error> DirWatch(int fd);
  std::expected<int, sigmaos::serr::Error> MountTree(std::shared_ptr<TendpointProto> ep, std::string tree, std::string mount);
  std::expected<bool, sigmaos::serr::Error> IsLocalMount(std::shared_ptr<TendpointProto> ep);
  std::expected<std::pair<std::vector<std::string>, std::vector<std::string>>, sigmaos::serr::Error> PathLastMount(std::string pn);
  std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> GetNamedEndpoint();
  std::expected<int, sigmaos::serr::Error> InvalidateNamedEndpointCacheEntryRealm(sigmaos::sigmap::types::Trealm realm);
  std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> GetNamedEndpointRealm(sigmaos::sigmap::types::Trealm realm);
  std::expected<int, sigmaos::serr::Error> NewRootMount(std::string pn, std::string mntname);
  std::expected<std::vector<std::string>, sigmaos::serr::Error> Mounts();
  std::expected<int, sigmaos::serr::Error> SetLocalMount(std::shared_ptr<TendpointProto>, sigmaos::sigmap::types::Tport port);
  // TODO: support MountPathClnt?
  //func (scc *SPProxyClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
  std::expected<int, sigmaos::serr::Error> Detach(std::string pn);
  std::expected<bool, sigmaos::serr::Error> Disconnected();
  std::expected<int, sigmaos::serr::Error> Disconnect(std::string pn);

  // ========== Endpoint manipulation ==========
  std::expected<int, sigmaos::serr::Error> RegisterEP(std::string pn, std::shared_ptr<TendpointProto> ep);

  // ========== ProcClnt API ==========
  std::expected<int, sigmaos::serr::Error> Started();
  std::expected<int, sigmaos::serr::Error> Exited(sigmaos::proc::Tstatus status, std::string &msg);
  std::expected<int, sigmaos::serr::Error> WaitEvict();

  // ========== Utility functions ==========
  std::thread StartWaitEvictThread();

  private:
  std::shared_ptr<sigmaos::io::conn::Conn> _conn;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans; std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  std::shared_ptr<sigmaos::proc::ProcEnv> _env;
  bool _disconnected;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void init_conn();
  void wait_for_eviction();
};

};
};
