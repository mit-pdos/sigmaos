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
      if (!_sp_clnt->ProcEnv()->GetUseShmem()) {
        _clnts[srv_id] = std::make_shared<sigmaos::rpc::Clnt>(
            chan, _sp_clnt->GetSPProxyChannel());
      } else {
        _clnts[srv_id] = std::make_shared<sigmaos::rpc::Clnt>(
            chan, _sp_clnt->GetSPProxyChannel(), _sp_clnt->GetShmemSegment());
      }
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

void Clnt::init_clnt(
    std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>
        result,
    uint32_t srv_id) {
  auto res = get_clnt(srv_id, true);
  if (!res.has_value()) {
    log(CACHECLNT_ERR, "Error init_clnt get_clnt ({}): {}", (int)srv_id,
        res.error().String());
    result->set_value(std::unexpected(res.error()));
  }
  result->set_value(0);
}

std::expected<int, sigmaos::serr::Error> Clnt::InitClnt(uint32_t srv_id) {
  auto res = get_clnt(srv_id, true);
  if (!res.has_value()) {
    log(CACHECLNT_ERR, "Error init_clnt get_clnt ({}): {}", (int)srv_id,
        res.error().String());
    return std::unexpected(res.error());
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::InitClnts(uint32_t last_srv_id) {
  std::vector<std::thread> init_threads;
  std::vector<
      std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>>
      init_promises;
  std::vector<std::future<std::expected<int, sigmaos::serr::Error>>>
      init_results;
  // Start client initializations in multiple threads
  for (uint32_t srv_id = 0; srv_id < last_srv_id; srv_id++) {
    init_promises.push_back(
        std::make_shared<
            std::promise<std::expected<int, sigmaos::serr::Error>>>());
    init_results.push_back(init_promises.at(srv_id)->get_future());
    init_threads.push_back(
        std::thread(&Clnt::init_clnt, this, init_promises.at(srv_id), srv_id));
  }
  for (int i = 0; i < init_threads.size(); i++) {
    init_threads.at(i).join();
    auto res = init_results.at(i).get();
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error init_clnts {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  return 0;
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

std::expected<
    std::shared_ptr<std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>,
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
  CacheMultiGetRep rep;
  CacheMultiGetReq req;
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
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  auto vals = std::make_shared<
      std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>(
      rep.lengths().size(), nullptr);
  uint64_t off = 0;
  for (int i = 0; i < vals->size(); i++) {
    uint64_t len = rep.lengths().at(i);
    vals->at(i) = std::make_shared<sigmaos::apps::cache::Value>(buf, off, len);
    off += len;
  }
  log(CACHECLNT, "MultiGet ok");
  return vals;
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

std::expected<
    std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
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
  auto kvs =
      std::make_shared<std::map<std::string, std::shared_ptr<std::string>>>();
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

std::expected<
    std::shared_ptr<std::map<
        uint32_t,
        std::shared_ptr<std::map<
            std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>,
    sigmaos::serr::Error>
Clnt::MultiDumpShard(uint32_t srv, std::vector<uint32_t> &shards) {
  log(CACHECLNT, "MultiDumpShard");
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(srv, true);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  TfenceProto fence;
  MultiShardReq req;
  req.set_allocated_fence(&fence);
  for (auto &shard : shards) {
    req.mutable_shards()->Add(shard);
  }
  MultiShardRep rep;
  Blob blob;
  auto iov = blob.mutable_iov();
  // Add a buffer to hold the output
  std::vector<std::shared_ptr<std::string>> shard_data;
  for (int i = 0; i < shards.size(); i++) {
    shard_data.push_back(std::make_shared<std::string>());
    iov->AddAllocated(shard_data[i].get());
  }
  rep.set_allocated_blob(&blob);
  auto shard_map = std::make_shared<std::map<
      uint32_t,
      std::shared_ptr<std::map<
          std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>();
  {
    auto res = rpcc->RPC("CacheSrv.MultiDumpShard", req, rep);
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
    for (auto &shard : shards) {
      (*shard_map)[shard] = std::make_shared<std::map<
          std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>();
    }
    int shard_idx = 0;
    int shard_off = 0;
    std::shared_ptr<std::string> shard_buf = shard_data[shard_idx];
    for (int i = 0; i < rep.keys().size(); i++) {
      auto k = rep.keys(i);
      auto v = std::make_shared<sigmaos::apps::cache::Value>(
          shard_buf, shard_off, rep.lens(i));
      auto shard = key2shard(k);
      (*((*shard_map)[shard]))[k] = v;
      shard_off += rep.lens(i);
      if (shard_off >= shard_buf->size()) {
        shard_off = 0;
        shard_idx++;
        while (shard_idx < shard_data.size() &&
               shard_data[shard_idx]->size() == 0) {
          shard_idx++;
        }
        if (shard_idx < shard_data.size()) {
          shard_buf = shard_data[shard_idx];
        }
      }
    }
  }
  log(CACHECLNT, "MultiDumpShard ok");
  return shard_map;
}

std::expected<int, sigmaos::serr::Error> Clnt::BatchFetchDelegatedRPCs(
    std::vector<uint64_t> &rpc_idxs, int n_iov) {
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
      log(CACHECLNT_ERR, "Error BatchFetchDelegatedRPCs: {}",
          res.error().String());
      return std::unexpected(res.error());
    }
  }
  log(CACHECLNT, "BatchFetchDelegatedRPCs ok: {}", rpc_idxs.size());
  return 0;
}

std::expected<
    std::shared_ptr<std::map<
        uint32_t,
        std::shared_ptr<std::map<
            std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>,
    sigmaos::serr::Error>
Clnt::DelegatedMultiDumpShard(uint64_t rpc_idx, std::vector<uint32_t> &shards) {
  log(CACHECLNT, "DelegatedMultiDumpShard({}) nshard {}", (int)rpc_idx,
      shards.size());
  std::shared_ptr<sigmaos::rpc::Clnt> rpcc;
  {
    auto res = get_clnt(0, false);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error get_clnt: {}", res.error().String());
      return std::unexpected(res.error());
    }
    rpcc = res.value();
  }
  MultiShardRep rep;
  Blob blob;
  auto iov = blob.mutable_iov();
  std::shared_ptr<std::vector<std::shared_ptr<std::string_view>>> shard_views =
      nullptr;
  std::vector<std::shared_ptr<std::string>> shard_bufs;
  if (_sp_clnt->ProcEnv()->GetUseShmem()) {
    shard_views =
        std::make_shared<std::vector<std::shared_ptr<std::string_view>>>();
    // Add buffer views to hold the output
    for (int i = 0; i < shards.size(); i++) {
      shard_views->push_back(std::make_shared<std::string_view>());
    }
  } else {
    // Add buffers to hold the output
    for (int i = 0; i < shards.size(); i++) {
      shard_bufs.push_back(std::make_shared<std::string>());
      iov->AddAllocated(shard_bufs[i].get());
    }
  }
  rep.set_allocated_blob(&blob);
  auto shard_map = std::make_shared<std::map<
      uint32_t,
      std::shared_ptr<std::map<
          std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>();
  {
    auto res = rpcc->DelegatedRPC(rpc_idx, rep, shard_views);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error DelegatedRPC: {}", res.error().String());
      return std::unexpected(res.error());
    }
    for (auto &shard : shards) {
      (*shard_map)[shard] = std::make_shared<std::map<
          std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>();
    }
    if (!_sp_clnt->ProcEnv()->GetUseShmem()) {
      shard_views =
          std::make_shared<std::vector<std::shared_ptr<std::string_view>>>();
      for (int i = 0; i < shard_bufs.size(); i++) {
        shard_views->push_back(std::make_shared<std::string_view>(
            shard_bufs.at(i)->data(), shard_bufs.at(i)->size()));
      }
    }
    auto start = GetCurrentTime();
    int shard_idx = 0;
    int shard_off = 0;
    auto shard_view = shard_views->at(0);
    for (int i = 0; i < rep.keys().size(); i++) {
      auto k = rep.keys(i);
      std::shared_ptr<sigmaos::apps::cache::Value> v = nullptr;
      if (_sp_clnt->ProcEnv()->GetUseShmem()) {
        v = std::make_shared<sigmaos::apps::cache::Value>(shard_view, shard_off,
                                                          rep.lens(i));
      } else {
        v = std::make_shared<sigmaos::apps::cache::Value>(
            shard_bufs.at(shard_idx), shard_off, rep.lens(i));
      }
      auto shard = key2shard(k);
      (*((*shard_map)[shard]))[k] = v;
      shard_off += rep.lens(i);
      if (shard_off >= shard_view->size()) {
        shard_off = 0;
        shard_idx++;
        while (shard_idx < shard_views->size() &&
               shard_views->at(shard_idx)->size() == 0) {
          shard_idx++;
        }
        if (shard_idx < shard_views->size()) {
          shard_view = shard_views->at(shard_idx);
        }
      }
    }
    log(PROXY_RPC_LAT, "CacheClnt.Construct map lat:{}ms", LatencyMS(start));
  }
  log(CACHECLNT, "DelegatedMultiDumpShard({}) ok", (int)rpc_idx);
  return shard_map;
}

std::expected<
    std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
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
  auto kvs =
      std::make_shared<std::map<std::string, std::shared_ptr<std::string>>>();
  {
    auto res = rpcc->DelegatedRPC(rpc_idx, rep);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
    for (auto &[k, v] : *rep.mutable_vals()) {
      (*kvs)[k] = std::make_shared<std::string>(std::move(v));
    }
  }
  log(CACHECLNT, "DelegatedDumpShard ok: {}", rpc_idx);
  return kvs;
}

std::expected<
    std::shared_ptr<std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>,
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
  std::shared_ptr<std::vector<std::shared_ptr<std::string_view>>> buf_views =
      nullptr;
  std::shared_ptr<std::string> buf;
  // Add a buffer, or a string view if using shared mem, to hold the output
  if (_sp_clnt->ProcEnv()->GetUseShmem()) {
    buf_views =
        std::make_shared<std::vector<std::shared_ptr<std::string_view>>>();
    buf_views->push_back(std::make_shared<std::string_view>());
  } else {
    buf = std::make_shared<std::string>();
    iov->AddAllocated(buf.get());
  }
  rep.set_allocated_blob(&blob);
  {
    auto res = rpcc->DelegatedRPC(rpc_idx, rep, buf_views);
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  auto vals = std::make_shared<
      std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>(
      rep.lengths().size(), nullptr);
  std::shared_ptr<std::string_view> buf_view;
  if (_sp_clnt->ProcEnv()->GetUseShmem()) {
    buf_view = buf_views->at(0);
  }
  uint64_t off = 0;
  for (int i = 0; i < vals->size(); i++) {
    uint64_t len = rep.lengths().at(i);
    if (_sp_clnt->ProcEnv()->GetUseShmem()) {
      vals->at(i) =
          std::make_shared<sigmaos::apps::cache::Value>(buf_view, off, len);
    } else {
      vals->at(i) =
          std::make_shared<sigmaos::apps::cache::Value>(buf, off, len);
    }
    off += len;
  }
  log(CACHECLNT, "DelegatedMultiGet ok");
  return vals;
}

};  // namespace apps::cache
};  // namespace sigmaos
