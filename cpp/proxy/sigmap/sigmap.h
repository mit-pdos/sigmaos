#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/demux/demux.h>
#include <serr/serr.h>
#include <rpc/rpc.h>
#include <proc/proc.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET_PN = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET
const std::string SPPROXYCLNT = "SPPROXYCLNT";
const std::string SPPROXYCLNT_ERR = "SPPROXYCLNT" + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt() {
    _env = sigmaos::proc::GetProcEnv();
    log(SPPROXYCLNT, "New clnt {}", _env->String());
    _conn = std::make_shared<sigmaos::io::conn::UnixConn>(SPPROXY_SOCKET_PN);
    _trans = std::make_shared<sigmaos::io::transport::Transport>(_conn);
    _demux = std::make_shared<sigmaos::io::demux::Clnt>(_trans);
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(_demux);
    log(SPPROXYCLNT, "Initializing proxy conn");
    // Initialize the sigmaproxyd connection
    init_conn();
  }

  ~Clnt() { Close(); }
  void Close() { 
    log(SPPROXYCLNT, "Close");
    _rpcc->Close(); 
    log(SPPROXYCLNT, "Done close");
  }

  std::expected<int, sigmaos::serr::Error> Test();

  // Stubs

  std::expected<uint64_t, sigmaos::serr::Error> CloseFD(int fd);
  std::expected<std::shared_ptr<TstatProto>, sigmaos::serr::Error> Stat(std::string pn);
  std::expected<int, sigmaos::serr::Error> Create(std::string pn, int perm, int mode);
  // TODO: wait type in Open?
  std::expected<int, sigmaos::serr::Error> Open(std::string pn, int mode /*, w sos.Twait*/);
  std::expected<int, sigmaos::serr::Error> Rename(std::string src, std::string dst);
  std::expected<int, sigmaos::serr::Error> Remove(std::string pn);
  std::expected<std::vector<unsigned char>, sigmaos::serr::Error> GetFile(std::string pn);
  std::expected<uint32_t, sigmaos::serr::Error> PutFile(std::string pn, int perm, int mode, std::vector<unsigned char> data, uint64_t offset, uint64_t leaseID);
  std::expected<uint32_t, sigmaos::serr::Error> Read(int fd, std::vector<unsigned char> b);
  std::expected<uint32_t, sigmaos::serr::Error> Pread(int fd, std::vector<unsigned char> b, uint64_t offset);
  // TODO: support PreadRdr?
  //func (scc *SPProxyClnt) PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
  std::expected<uint32_t, sigmaos::serr::Error> Write(int fd, std::vector<unsigned char> b);
  std::expected<int, sigmaos::serr::Error> Seek(int fd, uint64_t offset);
  // TODO: fence type in CreateLeased?
  std::expected<int, sigmaos::serr::Error> CreateLeased(std::string path, int perm, int mode, uint64_t leaseID/*, f sp.Tfence*/);
  std::expected<uint64_t, sigmaos::serr::Error> ClntID();
  // TODO: fence type in FenceDir?
  std::expected<int, sigmaos::serr::Error> FenceDir(std::string pn/*, f sp.Tfence*/);
  // TODO: support WriteFence?
  //func (scc *SPProxyClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
  std::expected<int, sigmaos::serr::Error> WriteRead(int fd, std::vector<std::vector<unsigned char>> in_iov, std::vector<std::vector<unsigned char>> out_iov);
  std::expected<int, sigmaos::serr::Error> DirWatch(int fd);
  std::expected<int, sigmaos::serr::Error> MountTree(std::shared_ptr<TendpointProto> ep, std::string tree, std::string mount);
  std::expected<bool, sigmaos::serr::Error> IsLocalMount(std::shared_ptr<TendpointProto> ep);
  std::expected<std::pair<std::string, std::string>, sigmaos::serr::Error> PathLastMount(std::string pn);
  std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> GetNamedEndpoint();
  std::expected<int, sigmaos::serr::Error> InvalidateNamedEndpointCacheEntryRealn(std::string realm);
  std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> GetNamedEndpointRealm(std::string realm);
  std::expected<int, sigmaos::serr::Error> NewRootMount(std::string pn, std::string mntname);
  std::expected<std::vector<std::string>, sigmaos::serr::Error> Mounts();
  std::expected<int, sigmaos::serr::Error> SetLocalMount(std::shared_ptr<TendpointProto>, int port);
  // TODO: support MountPathClnt?
  //func (scc *SPProxyClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
  std::expected<int, sigmaos::serr::Error> Detach(std::string pn);
  std::expected<bool, sigmaos::serr::Error> Disconnected();
  std::expected<int, sigmaos::serr::Error> Disconnect(std::string pn);

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  std::shared_ptr<sigmaos::proc::ProcEnv> _env;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void init_conn();
};

};
};
