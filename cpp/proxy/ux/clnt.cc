#include <proxy/ux/clnt.h>

namespace sigmaos {
namespace proxy::ux {

bool Clnt::_l = sigmaos::util::log::init_logger(UXCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(UXCLNT_ERR);

Clnt::Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt,
           std::string svc_pn)
    : _sp_clnt(sp_clnt) {
  auto chan =
      std::make_shared<sigmaos::rpc::spchannel::Channel>(svc_pn, sp_clnt);
  _rpcc = std::make_shared<sigmaos::rpc::Clnt>(
      chan, sp_clnt->GetSPProxyChannel(), sp_clnt->GetShmemSegment());
}

std::expected<std::shared_ptr<sigmaos::proxy::buf::DataBuf>,
              sigmaos::serr::Error>
Clnt::GetFile(std::string path) {
  UXReq req;
  UXRep rep;
  req.set_path(path);

  bool use_shmem = _sp_clnt->GetUseShmemWriteread();
  log(UXCLNT, "GetFile path:{} shmem:{}", path, use_shmem);
  std::shared_ptr<std::vector<std::shared_ptr<std::string_view>>> views;
  std::shared_ptr<std::string> s;
  Blob blob;

  if (use_shmem) {
    views = std::make_shared<std::vector<std::shared_ptr<std::string_view>>>();
    views->push_back(std::make_shared<std::string_view>());
  } else {
    s = std::make_shared<std::string>();
    blob.mutable_iov()->AddAllocated(s.get());
    rep.set_allocated_blob(&blob);
  }

  auto res = _rpcc->RPC("UXRpcAPI.GetFile", req, rep, views);
  if (!use_shmem) {
    auto _ = rep.release_blob();
  }
  if (!res.has_value()) {
    log(UXCLNT_ERR, "Err GetFile: {}", res.error().String());
    return std::unexpected(res.error());
  }

  std::shared_ptr<sigmaos::proxy::buf::DataBuf> dbuf;
  if (use_shmem) {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(*views->at(0));
  } else {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(std::move(s));
  }
  log(UXCLNT, "GetFile ok path:{} len:{} shmem:{}", path, dbuf->size(),
      use_shmem);
  return dbuf;
}

std::expected<std::shared_ptr<sigmaos::proxy::buf::DataBuf>,
              sigmaos::serr::Error>
Clnt::DelegatedGetFile(uint64_t rpc_idx) {
  log(UXCLNT, "DelegatedGetFile rpc_idx:{}", rpc_idx);
  UXRep rep;
  Blob blob;
  std::shared_ptr<std::string> owned;
  std::shared_ptr<std::vector<std::shared_ptr<std::string_view>>> views =
      nullptr;
  if (_sp_clnt->GetShmemEnabled()) {
    views = std::make_shared<std::vector<std::shared_ptr<std::string_view>>>();
    views->push_back(std::make_shared<std::string_view>());
  } else {
    owned = std::make_shared<std::string>();
    blob.mutable_iov()->AddAllocated(owned.get());
  }
  rep.set_allocated_blob(&blob);
  auto res = _rpcc->DelegatedRPC(rpc_idx, rep, views);
  if (!res.has_value()) {
    log(UXCLNT_ERR, "Err DelegatedGetFile: {}", res.error().String());
    return std::unexpected(res.error());
  }
  std::shared_ptr<sigmaos::proxy::buf::DataBuf> dbuf;
  if (_sp_clnt->GetShmemEnabled()) {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(*views->at(0));
  } else {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(std::move(owned));
  }
  log(UXCLNT, "DelegatedGetFile ok rpc_idx:{} len:{}", rpc_idx, dbuf->size());
  return dbuf;
}

std::expected<int, sigmaos::serr::Error> Clnt::PutFile(std::string path,
                                                       std::string* data) {
  log(UXCLNT, "PutFile path:{} len:{}", path, data->size());
  UXReq req;
  UXRep rep;
  req.set_path(path);
  Blob blob;
  blob.mutable_iov()->AddAllocated(data);
  req.set_allocated_blob(&blob);
  auto res = _rpcc->RPC("UXRpcAPI.PutFile", req, rep);
  {
    auto _ = req.release_blob()->mutable_iov()->ReleaseLast();
  }
  if (!res.has_value()) {
    log(UXCLNT_ERR, "Err PutFile: {}", res.error().String());
    return std::unexpected(res.error());
  }
  log(UXCLNT, "PutFile ok path:{}", path);
  return 0;
}

}  // namespace proxy::ux
}  // namespace sigmaos
