#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

namespace sigmaos {
namespace proxy::sigmap {

bool Clnt::_l = sigmaos::util::log::init_logger(SPPROXYCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(SPPROXYCLNT_ERR);

void Clnt::init_conn() {
  SigmaInitReq req;
  SigmaErrRep rep;
  // Set the proc env proto
  req.set_allocated_procenvproto(_env->GetProto());
  // Execute the RPC
  auto res = _rpcc->RPC("SPProxySrvAPI.Init", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    throw std::runtime_error(std::format("Err rpc: {}", res.error().String()));
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    throw std::runtime_error(std::format("init rpc error: {}", rep.err().DebugString()));
  }
  log(SPPROXYCLNT, "Init RPC successful");
  // Make sure to release the proc env proto pointer so it isn't destroyed
  req.release_procenvproto();
}

std::expected<int, sigmaos::serr::Error> Clnt::Test() {
  auto res = ClntID();
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return res;
  }
  log(TEST, "Test successful");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::CloseFD(int fd) {
  log(SPPROXYCLNT, "CloseFD: {}", fd);
  SigmaCloseReq req;
  SigmaErrRep rep;
  auto res = _rpcc->RPC("SPProxySrvAPI.CloseFd", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "CloseFD done: {}", fd);
  return 0;
}

std::expected<std::shared_ptr<TstatProto>, sigmaos::serr::Error> Clnt::Stat(std::string pn) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::Create(std::string pn, int perm, int mode) {
  throw std::runtime_error("unimplemented");
}

// TODO: wait type in Open?
std::expected<int, sigmaos::serr::Error> Clnt::Open(std::string pn, int mode /*, w sos.Twait*/) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::Rename(std::string src, std::string dst) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::Remove(std::string pn) {
  throw std::runtime_error("unimplemented");
}

std::expected<std::vector<unsigned char>, sigmaos::serr::Error> Clnt::GetFile(std::string pn) {
  throw std::runtime_error("unimplemented");
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::PutFile(std::string pn, int perm, int mode, std::vector<unsigned char> data, uint64_t offset, uint64_t leaseID) {
  throw std::runtime_error("unimplemented");
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::Read(int fd, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented");
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::Pread(int fd, std::vector<unsigned char> b, uint64_t offset) {
  throw std::runtime_error("unimplemented");
}

// TODO: support PreadRdr?
//func (scc *SPProxyClnt) PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
std::expected<uint32_t, sigmaos::serr::Error> Clnt::Write(int fd, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::Seek(int fd, uint64_t offset) {
  throw std::runtime_error("unimplemented");
}

// TODO: fence type in CreateLeased?
std::expected<int, sigmaos::serr::Error> Clnt::CreateLeased(std::string path, int perm, int mode, uint64_t leaseID/*, f sp.Tfence*/) {
  throw std::runtime_error("unimplemented");
}

std::expected<uint64_t, sigmaos::serr::Error> Clnt::ClntID() {
  log(SPPROXYCLNT, "ClntID");
  SigmaNullReq req;
  SigmaClntIdRep rep;
  auto res = _rpcc->RPC("SPProxySrvAPI.ClntId", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  log(SPPROXYCLNT, "ClntID done: {}", rep.clntid());
  return rep.clntid();
}

// TODO: fence type in FenceDir?
std::expected<int, sigmaos::serr::Error> Clnt::FenceDir(std::string pn/*, f sp.Tfence*/) {
  throw std::runtime_error("unimplemented");
}

// TODO: support WriteFence?
//func (scc *SPProxyClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {
std::expected<int, sigmaos::serr::Error> Clnt::WriteRead(int fd, std::vector<std::vector<unsigned char>> in_iov, std::vector<std::vector<unsigned char>> out_iov) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::DirWatch(int fd) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::MountTree(std::shared_ptr<TendpointProto> ep, std::string tree, std::string mount) {
  throw std::runtime_error("unimplemented");
}

std::expected<bool, sigmaos::serr::Error> Clnt::IsLocalMount(std::shared_ptr<TendpointProto> ep) {
  throw std::runtime_error("unimplemented");
}

std::expected<std::pair<std::string, std::string>, sigmaos::serr::Error> Clnt::PathLastMount(std::string pn) {
  throw std::runtime_error("unimplemented");
}

std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> Clnt::GetNamedEndpoint() {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::InvalidateNamedEndpointCacheEntryRealn(std::string realm) {
  throw std::runtime_error("unimplemented");
}

std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> Clnt::GetNamedEndpointRealm(std::string realm) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::NewRootMount(std::string pn, std::string mntname) {
  throw std::runtime_error("unimplemented");
}

std::expected<std::vector<std::string>, sigmaos::serr::Error> Clnt::Mounts() {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::SetLocalMount(std::shared_ptr<TendpointProto>, int port) {
  throw std::runtime_error("unimplemented");
}

// TODO: support MountPathClnt?
//func (scc *SPProxyClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
std::expected<int, sigmaos::serr::Error> Clnt::Detach(std::string pn) {
  throw std::runtime_error("unimplemented");
}

std::expected<bool, sigmaos::serr::Error> Clnt::Disconnected() {
  throw std::runtime_error("unimplemented");
}

std::expected<int, sigmaos::serr::Error> Clnt::Disconnect(std::string pn) {
  throw std::runtime_error("unimplemented");
}


};
};
