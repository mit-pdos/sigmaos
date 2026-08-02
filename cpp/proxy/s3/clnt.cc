#include <proxy/s3/clnt.h>

namespace sigmaos {
namespace proxy::s3 {

bool Clnt::_l = sigmaos::util::log::init_logger(S3CLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(S3CLNT_ERR);

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
Clnt::GetObject(std::string bucket, std::string key, bool cache) {
  S3Req req;
  S3Rep rep;
  req.set_bucket(bucket);
  req.set_key(key);
  req.set_cache(cache);

  bool use_shmem = _sp_clnt->GetUseShmemWriteread();
  log(S3CLNT, "GetObject bucket:{} key:{} shmem:{}", bucket, key, use_shmem);
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

  auto res = _rpcc->RPC("S3RpcAPI.GetObject", req, rep, views);
  if (!use_shmem) {
    auto _ = rep.release_blob();
  }
  if (!res.has_value()) {
    log(S3CLNT_ERR, "Err GetObject: {}", res.error().String());
    return std::unexpected(res.error());
  }

  std::shared_ptr<sigmaos::proxy::buf::DataBuf> dbuf;
  if (use_shmem) {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(*views->at(0));
  } else {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(std::move(s));
  }
  log(S3CLNT, "GetObject ok bucket:{} key:{} len:{} shmem:{}", bucket, key,
      dbuf->size(), use_shmem);
  return dbuf;
}

std::expected<std::shared_ptr<sigmaos::proxy::buf::DataBuf>,
              sigmaos::serr::Error>
Clnt::DelegatedGetObject(uint64_t rpc_idx) {
  log(S3CLNT, "DelegatedGetObject rpc_idx:{}", rpc_idx);
  S3Rep rep;
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
    log(S3CLNT_ERR, "Err DelegatedGetObject: {}", res.error().String());
    return std::unexpected(res.error());
  }
  std::shared_ptr<sigmaos::proxy::buf::DataBuf> dbuf;
  if (_sp_clnt->GetShmemEnabled()) {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(*views->at(0));
  } else {
    dbuf = std::make_shared<sigmaos::proxy::buf::DataBuf>(std::move(owned));
  }
  log(S3CLNT, "DelegatedGetObject ok rpc_idx:{} len:{}", rpc_idx, dbuf->size());
  return dbuf;
}

std::expected<int, sigmaos::serr::Error> Clnt::PutObject(std::string bucket,
                                                         std::string key,
                                                         std::string* data) {
  log(S3CLNT, "PutObject bucket:{} key:{} len:{}", bucket, key, data->size());
  S3Req req;
  S3Rep rep;
  req.set_bucket(bucket);
  req.set_key(key);
  Blob blob;
  auto iov = blob.mutable_iov();
  iov->AddAllocated(data);
  req.set_allocated_blob(&blob);
  auto res = _rpcc->RPC("S3RpcAPI.PutObject", req, rep);
  {
    auto _ = req.release_blob();
  }
  if (!res.has_value()) {
    log(S3CLNT_ERR, "Err PutObject: {}", res.error().String());
    return std::unexpected(res.error());
  }
  log(S3CLNT, "PutObject ok bucket:{} key:{}", bucket, key);
  return 0;
}

}  // namespace proxy::s3
}  // namespace sigmaos
