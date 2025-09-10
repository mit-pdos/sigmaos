#include <apps/cache/clnt.h>

namespace sigmaos {
namespace apps::cache {

bool Clnt::_l = sigmaos::util::log::init_logger(CACHECLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(CACHECLNT_ERR);

std::expected<std::shared_ptr<sigmaos::rpc::Clnt>, sigmaos::serr::Error>
Clnt::get_clnt(int srv_id, bool initialize) {
  // Ensure we don't create duplicate clients
  std::lock_guard<std::mutex> guard(_mu);
  // If client does not exist,
  if (!_clnts.contains(srv_id)) {
    {
      // Create a sigmap RPC channel to the server via the sigmaproxy
      log(CACHECLNT, "Create channel (with lazy initialization)");
      std::string srv_pn = _svc_pn_base + "/" + std::to_string(srv_id);
      auto chan =
          std::make_shared<sigmaos::rpc::spchannel::Channel>(srv_pn, _sp_clnt);
      log(CACHECLNT, "Create RPC client");
      // Create an RPC client from the channel
      _clnts[srv_id] = std::make_shared<sigmaos::rpc::Clnt>(
          chan, _sp_clnt->GetSPProxyChannel());
      log(CACHECLNT, "Successfully created client srv_id:{}", srv_id);
    }
  } else {
    log(CACHECLNT, "Successfully got existing client srv_id:{}", srv_id);
  }
  auto clnt = _clnts.at(srv_id);
  if (initialize && !clnt->GetChannel()->IsInitialized()) {
    std::string srv_pn = _svc_pn_base + "/" + std::to_string(srv_id);
    auto cache_pair = _sp_clnt->ProcEnv()->GetCachedEndpoint(srv_pn);
    auto ep = cache_pair.first;
    bool ok = cache_pair.second;
    if (ok) {
      // Mount the cache server
      log(CACHECLNT, "Mount cached EP for cache server {}: {}", srv_id,
          ep->ShortDebugString());
      //
      {
        auto res = _sp_clnt->MountTree(ep, sigmaos::rpc::RPC,
                                       srv_pn + "/" + sigmaos::rpc::RPC);
        if (!res.has_value()) {
          log(CACHECLNT_ERR, "Error MountTree srv_id {}: {}", srv_id,
              res.error().String());
          return std::unexpected(res.error());
        }
        log(CACHECLNT, "Mounted RPC channel for srv_id:{}", srv_id);
      }
    }
    // Initialize the channel
    auto res = clnt->GetChannel()->Init();
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error initialize channel: {}", res.error().String());
      return std::unexpected(res.error());
    }
    log(CACHECLNT, "Initialized RPC channel for client  ofsrv_id:{}", srv_id);
  }
  return clnt;
}

std::expected<int, sigmaos::serr::Error> Clnt::Get(
    std::string key, std::shared_ptr<std::string> val) {
  log(CACHECLNT, "Get: {}", key);
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(key2server(key, _nsrv), true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  CacheRep rep;
  CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
  {
    auto res = rpcc->RPC("CacheSrv.Get", req, rep);
    {
      auto _ = req.release_fence();
    }
    auto start = GetCurrentTime();
    *val = rep.value();
    log(PROXY_RPC_LAT, "Set val to reply value lat:{}ms", LatencyMS(start));
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  log(CACHECLNT, "Get ok: {} -> {}b", key, val->size());
  return 0;
}

std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>,
              sigmaos::serr::Error>
Clnt::MultiGet(uint32_t srv_id, std::vector<std::string> &keys) {
  log(CACHECLNT, "MultiGet nkey {}", keys.size());
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(srv_id, true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  CacheMultiGetRep rep;
  CacheMultiGetReq req;
  req.set_allocated_fence(&fence);
  Blob blob;
  auto iov = blob.mutable_iov();
  // Add a buffer to hold the output
  auto buf = std::make_shared<std::string>();
  iov->AddAllocated(buf.get());
  auto gets = req.mutable_gets();
  for (int i = 0; i < keys.size(); i++) {
    auto get = gets->Add();
    get->set_key(keys.at(i));
    get->set_shard(key2shard(keys.at(i)));
  }
  rep.set_allocated_blob(&blob);
  {
    auto res = rpcc->RPC("CacheSrv.MultiGet", req, rep);
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  std::vector<uint64_t> lengths(rep.lengths().size(), 0);
  for (int i = 0; i < lengths.size(); i++) {
    lengths[i] = rep.lengths().at(i);
  }
  log(CACHECLNT, "MultiGet ok");
  return std::make_pair(lengths, buf);
}

std::expected<int, sigmaos::serr::Error> Clnt::Put(
    std::string key, std::shared_ptr<std::string> val) {
  log(CACHECLNT, "Put: {} -> {}b", key, val->size());
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(key2server(key, _nsrv), true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  CacheRep rep;
  CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
  req.set_allocated_value(val.get());
  {
    auto res = rpcc->RPC("CacheSrv.Put", req, rep);
    {
      auto _ = req.release_value();
    }
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Put: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  log(CACHECLNT, "Put ok: {} -> {}b", key, val->size());
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Delete(std::string key) {
  log(CACHECLNT, "Delete: {}", key);
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(key2server(key, _nsrv), true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  CacheRep rep;
  CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
  {
    auto res = rpcc->RPC("CacheSrv.Delete", req, rep);
    {
      auto _ = req.release_value();
    }
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Delete: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  log(CACHECLNT, "Delete ok: {}", key);
  return 0;
}

std::expected<std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
              sigmaos::serr::Error>
Clnt::DumpShard(uint32_t shard, bool empty) {
  log(CACHECLNT, "DumpShard: {}", shard);
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    uint32_t srv = shard % _nsrv;
    auto res = get_clnt(srv, true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  ShardData rep;
  ShardReq req;
  req.set_allocated_fence(&fence);
  req.set_shard(shard);
  req.set_empty(empty);
  auto kvs = std::make_shared<std::map<std::string, std::shared_ptr<std::string>>>();
  {
    auto res = rpcc->RPC("CacheSrv.DumpShard", req, rep);
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
    for (const auto &[k, v] : rep.vals()) {
      (*kvs)[k] = std::make_shared<std::string>(v);
    }
  }
  log(CACHECLNT, "DumpShard ok: {}", shard);
  return kvs;
}

std::expected<int, sigmaos::serr::Error>
Clnt::BatchFetchDelegatedRPCs(std::vector<uint64_t> &rpc_idxs, int n_iov) {
  log(CACHECLNT, "BatchFetchDelegatedRPCs: {}", rpc_idxs.size());
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(0, false);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  {
    auto res = rpcc->BatchFetchDelegatedRPCs(rpc_idxs, n_iov);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error BatchFetchDelegatedRPCs: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  log(CACHECLNT, "BatchFetchDelegatedRPCs ok: {}", rpc_idxs.size());
  return 0;
}

std::expected<std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
              sigmaos::serr::Error>
Clnt::DelegatedDumpShard(uint64_t rpc_idx) {
  log(CACHECLNT, "DelegatedDumpShard: {}", rpc_idx);
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(0, false);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  ShardData rep;
  auto kvs = std::make_shared<std::map<std::string, std::shared_ptr<std::string>>>();
  {
    auto res = rpcc->DelegatedRPC(rpc_idx, rep);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
    for (const auto &[k, v] : rep.vals()) {
      (*kvs)[k] = std::make_shared<std::string>(v);
    }
  }
  log(CACHECLNT, "DelegatedDumpShard ok: {}", rpc_idx);
  return kvs;
}

std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>,
              sigmaos::serr::Error>
Clnt::DelegatedMultiGet(uint64_t rpc_idx) {
  log(CACHECLNT, "Delegated MultiGet rpc_idx {}", (int)rpc_idx);
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(0, false);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  CacheMultiGetRep rep;
  Blob blob;
  auto iov = blob.mutable_iov();
  // Add a buffer to hold the output
  auto buf = std::make_shared<std::string>();
  iov->AddAllocated(buf.get());
  rep.set_allocated_blob(&blob);
  {
    auto res = rpcc->DelegatedRPC(rpc_idx, rep);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  std::vector<uint64_t> lengths(rep.lengths().size(), 0);
  for (int i = 0; i < lengths.size(); i++) {
    lengths[i] = rep.lengths().at(i);
  }
  log(CACHECLNT, "DelegatedMultiGet ok");
  return std::make_pair(lengths, buf);
}

};  // namespace apps::cache
};  // namespace sigmaos
