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
  auto _ = req.release_procenvproto();
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
  req.set_fd(fd);
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
  log(SPPROXYCLNT, "Stat: {}", pn);
  SigmaPathReq req;
  SigmaStatRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.Stat", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Stat done: {}", pn);
  return std::make_shared<TstatProto>(rep.stat());
}

std::expected<int, sigmaos::serr::Error> Clnt::Create(std::string pn, int perm, int mode) {
  log(SPPROXYCLNT, "Create: {} {} {}", pn, perm, mode);
  SigmaCreateReq req;
  SigmaFdRep rep;
  req.set_path(pn);
  req.set_perm(perm);
  req.set_mode(mode);
  auto res = _rpcc->RPC("SPProxySrvAPI.Create", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Create done: {} {} {}", pn, perm, mode);
  return rep.fd();
}

std::expected<int, sigmaos::serr::Error> Clnt::Open(std::string pn, int mode, bool wait) {
  log(SPPROXYCLNT, "Open: {} {} {}", pn, mode, wait);
  SigmaCreateReq req;
  SigmaFdRep rep;
  req.set_path(pn);
  req.set_mode(mode);
  req.set_wait(wait);
  auto res = _rpcc->RPC("SPProxySrvAPI.Open", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Open done: {} {} {}", pn, mode, wait);
  return rep.fd();
}

std::expected<int, sigmaos::serr::Error> Clnt::Rename(std::string src, std::string dst) {
  log(SPPROXYCLNT, "Rename: {} {}", src, dst);
  SigmaRenameReq req;
  SigmaErrRep rep;
  req.set_src(src);
  req.set_dst(dst);
  auto res = _rpcc->RPC("SPProxySrvAPI.Rename", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Rename done: {} {}", src, dst);
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Remove(std::string pn) {
  log(SPPROXYCLNT, "Remove: {}", pn);
  SigmaPathReq req;
  SigmaErrRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.Remove", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Remove done: {}", pn);
  return 0;
}

std::expected<std::vector<unsigned char>, sigmaos::serr::Error> Clnt::GetFile(std::string pn) {
  log(SPPROXYCLNT, "GetFile: {}", pn);
  SigmaPathReq req;
  SigmaDataRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.GetFile", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "GetFile done: {}", pn);
  // TODO: implement blobs
  throw std::runtime_error("unimplemented: blobs");
  std::vector<unsigned char> b;
  return b;
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::PutFile(std::string pn, int perm, int mode, std::vector<unsigned char> data, uint64_t offset, uint64_t leaseID) {
  throw std::runtime_error("unimplemented: blobs");
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::Read(int fd, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented: blobs");
}

std::expected<uint32_t, sigmaos::serr::Error> Clnt::Pread(int fd, std::vector<unsigned char> b, uint64_t offset) {
  throw std::runtime_error("unimplemented: blobs");
}

// TODO: support PreadRdr?
//func (scc *SPProxyClnt) PreadRdr(fd int, o sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
std::expected<uint32_t, sigmaos::serr::Error> Clnt::Write(int fd, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented: blobs");
}

std::expected<int, sigmaos::serr::Error> Clnt::Seek(int fd, uint64_t offset) {
  log(SPPROXYCLNT, "Seek: {} {}", fd, offset);
  SigmaSeekReq req;
  SigmaErrRep rep;
  req.set_fd(fd);
  req.set_offset(offset);
  auto res = _rpcc->RPC("SPProxySrvAPI.Seek", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Seek done: {} {}", fd, offset);
  return 0;
}

// TODO: fence type in CreateLeased?
std::expected<int, sigmaos::serr::Error> Clnt::CreateLeased(std::string pn, int perm, int mode, uint64_t leaseID/*, f sp.Tfence*/) {
  throw std::runtime_error("unimplemented: tfence");
  log(SPPROXYCLNT, "CreateLeased: {} {} {} {}", pn, perm, mode, leaseID);
  SigmaCreateReq req;
  SigmaFdRep rep;
  req.set_path(pn);
  req.set_perm(perm);
  req.set_mode(mode);
  req.set_mode(mode);
  req.set_leaseid(leaseID);
  auto res = _rpcc->RPC("SPProxySrvAPI.CreateLeased", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "CreateLeased done: {} {} {} {}", pn, perm, mode, leaseID);
  return 0;
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
  throw std::runtime_error("unimplemented: tfence");
  log(SPPROXYCLNT, "FenceDir: {}", pn);
  SigmaFenceReq req;
  SigmaErrRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.FenceDir", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "FenceDir done: {}", pn);
  return 0;
}

// TODO: support WriteFence?
//func (scc *SPProxyClnt) WriteFence(fd int, d []byte, f sp.Tfence) (sp.Tsize, error) {

std::expected<int, sigmaos::serr::Error> Clnt::WriteRead(int fd, std::vector<std::vector<unsigned char>> in_iov, std::vector<std::vector<unsigned char>> out_iov) {
  throw std::runtime_error("unimplemented: blob");
  log(SPPROXYCLNT, "WriteRead: {}", fd);
  SigmaWriteReq req;
  SigmaDataRep rep;
  req.set_fd(fd);
  auto res = _rpcc->RPC("SPProxySrvAPI.WriteRead", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "WriteRead done: {}", fd);
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::DirWatch(int fd) {
  log(SPPROXYCLNT, "DirWatch: {}", fd);
  SigmaReadReq req;
  SigmaFdRep rep;
  req.set_fd(fd);
  auto res = _rpcc->RPC("SPProxySrvAPI.DirWatch", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "DirWatch done: {}", fd);
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::MountTree(std::shared_ptr<TendpointProto> ep, std::string tree, std::string mount) {
  log(SPPROXYCLNT, "MountTree: {} {} {}", ep->DebugString(), tree, mount);
  SigmaMountTreeReq req;
  SigmaErrRep rep;
  req.set_allocated_endpoint(ep.get());
  req.set_tree(tree);
  req.set_mountname(mount);
  auto res = _rpcc->RPC("SPProxySrvAPI.MountTree", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  auto _ = req.release_endpoint();
  log(SPPROXYCLNT, "MountTree done: {} {} {}", ep->DebugString(), tree, mount);
  return 0;
}

std::expected<bool, sigmaos::serr::Error> Clnt::IsLocalMount(std::shared_ptr<TendpointProto> ep) {
  log(SPPROXYCLNT, "IsLocalMount: {}", ep->DebugString());
  SigmaMountReq req;
  SigmaMountRep rep;
  req.set_allocated_endpoint(ep.get());
  auto res = _rpcc->RPC("SPProxySrvAPI.IsLocalMount", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  auto _ = req.release_endpoint();
  log(SPPROXYCLNT, "IsLocalMount done: {}", ep->DebugString());
  return rep.local();
}

std::expected<std::pair<std::vector<std::string>, std::vector<std::string>>, sigmaos::serr::Error> Clnt::PathLastMount(std::string pn) {
  log(SPPROXYCLNT, "PathLastMount: {}", pn);
  SigmaPathReq req;
  SigmaLastMountRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.PathLastMount", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "PathLastMount done: {}", pn);
  std::vector<std::string> p1(rep.path1().size());
  for (int i = 0; i < rep.path1().size(); i++) {
    p1[i] = std::string(rep.path1().Get(i));
  }
  std::vector<std::string> p2(rep.path2().size());
  for (int i = 0; i < rep.path2().size(); i++) {
    p2[i] = std::string(rep.path2().Get(i));
  }
  return std::make_pair(p1, p2);
}

std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> Clnt::GetNamedEndpoint() {
  return GetNamedEndpointRealm(_env->GetRealm());
}

std::expected<int, sigmaos::serr::Error> Clnt::InvalidateNamedEndpointCacheEntryRealm(std::string realm) {
  log(SPPROXYCLNT, "InvalidateNamedEndpointCacheEntryRealm: {}", realm);
  SigmaRealmReq req;
  SigmaMountRep rep;
  req.set_realmstr(realm);
  auto res = _rpcc->RPC("SPProxySrvAPI.InvalidateNamedEndpointCacheEntryRealm", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "InvalidateNamedEndpointCacheEntryRealm done: {}", realm);
  return 0;
}

std::expected<std::shared_ptr<TendpointProto>, sigmaos::serr::Error> Clnt::GetNamedEndpointRealm(std::string realm) {
  log(SPPROXYCLNT, "GetNamedEndpointRealm: {}", realm);
  SigmaRealmReq req;
  SigmaMountRep rep;
  req.set_realmstr(realm);
  auto res = _rpcc->RPC("SPProxySrvAPI.GetNamedEndpointRealm", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "GetNamedEndpointRealm done: {}", realm);
  return std::make_shared<TendpointProto>(rep.endpoint());
}

std::expected<int, sigmaos::serr::Error> Clnt::NewRootMount(std::string pn, std::string mntname) {
  log(SPPROXYCLNT, "NewRootMount: {} {}", pn, mntname);
  SigmaMountTreeReq req;
  SigmaErrRep rep;
  req.set_tree(pn);
  req.set_mountname(mntname);
  auto res = _rpcc->RPC("SPProxySrvAPI.NewRootMount", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "NewRootMount done: {} {}", pn, mntname);
  return 0;
}

std::expected<std::vector<std::string>, sigmaos::serr::Error> Clnt::Mounts() {
  log(SPPROXYCLNT, "Mounts");
  SigmaNullReq req;
  SigmaMountsRep rep;
  auto res = _rpcc->RPC("SPProxySrvAPI.Mounts", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Mounts done");
  std::vector<std::string> mounts(rep.endpoints().size());
  for (int i = 0; i < rep.endpoints().size(); i++) {
    mounts[i] = rep.endpoints().Get(i);
  }
  return mounts;
}

std::expected<int, sigmaos::serr::Error> Clnt::SetLocalMount(std::shared_ptr<TendpointProto>, int port) {
  throw std::runtime_error("unimplemented (in go version too)");
}

// TODO: support MountPathClnt?
//func (scc *SPProxyClnt) MountPathClnt(path string, clnt sos.PathClntAPI) error {
std::expected<int, sigmaos::serr::Error> Clnt::Detach(std::string pn) {
  log(SPPROXYCLNT, "Detach {}", pn);
  SigmaPathReq req;
  SigmaErrRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.Detach", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Detach done");
  return 0;
}

std::expected<bool, sigmaos::serr::Error> Clnt::Disconnected() {
  return _disconnected;
}

std::expected<int, sigmaos::serr::Error> Clnt::Disconnect(std::string pn) {
  _disconnected = true;
  log(SPPROXYCLNT, "Disconnect {}", pn);
  SigmaPathReq req;
  SigmaErrRep rep;
  req.set_path(pn);
  auto res = _rpcc->RPC("SPProxySrvAPI.Disconnect", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  if (rep.err().errcode() != sigmaos::serr::Terror::TErrNoError) {
    return std::unexpected(sigmaos::serr::Error((sigmaos::serr::Terror) rep.err().errcode(), rep.err().obj()));
  }
  log(SPPROXYCLNT, "Disconnect done {}", pn);
  return 0;
}


std::expected<int, sigmaos::serr::Error> Clnt::Started() {
  log(SPPROXYCLNT, "Started");
  SigmaNullReq req;
  SigmaErrRep rep;
  auto res = _rpcc->RPC("SPProxySrvAPI.Started", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  log(SPPROXYCLNT, "Started done");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Exited() {
  log(SPPROXYCLNT, "Exited");
  SigmaNullReq req;
  SigmaErrRep rep;
  auto res = _rpcc->RPC("SPProxySrvAPI.Exited", req, rep);
  if (!res.has_value()) {
    log(SPPROXYCLNT_ERR, "Err RPC: {}", res.error());
    return std::unexpected(res.error());
  }
  log(SPPROXYCLNT, "Exited done");
  return 0;
}

};
};
