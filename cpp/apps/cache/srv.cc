#include <apps/cache/srv.h>

namespace sigmaos {
namespace apps::cache {

bool Srv::_l = sigmaos::util::log::init_logger(CACHESRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(CACHESRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::Get(
    std::shared_ptr<google::protobuf::Message> preq,
    std::shared_ptr<google::protobuf::Message> prep) {
  // Register that a request was received
  _perf->TptTick(1.0);
  auto start = GetCurrentTime();
  auto req = dynamic_pointer_cast<CacheReq>(preq);
  auto rep = dynamic_pointer_cast<CacheRep>(prep);
  auto req_cnt = _req_cnt++;
  auto key = req->key();
  log(CACHESRV, "CacheSrv.Get req({}) key={}", req_cnt, key);
  // Take the lock
  std::lock_guard<std::mutex> guard(_mu);
  // If the shard isn't present, return an error
  if (!_cache.contains(req->shard())) {
    log(CACHESRV_ERR, "CacheSrv.Get rep({}) shard {} not found", req_cnt,
        req->shard());
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrNotfound,
                             std::format("shard {}", req->shard())));
  }
  std::shared_ptr<std::string> val;
  // Get the shard
  auto s = _cache.at(req->shard());
  {
    auto res = s->Get(key);
    if (!res.has_value()) {
      log(CACHESRV_ERR, "CacheSrv.Get rep({}) key {} not found", req_cnt, key);
      return std::unexpected(res.error());
    }
    val = res.value();
  }
  rep->set_value(*val);
  log(CACHESRV, "CacheSrv.Get rep({}) latency={:0.3f}ms", req_cnt,
      LatencyMS(start));
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Put(
    std::shared_ptr<google::protobuf::Message> preq,
    std::shared_ptr<google::protobuf::Message> prep) {
  // Register that a request was received
  _perf->TptTick(1.0);
  auto start = GetCurrentTime();
  auto req = dynamic_pointer_cast<CacheReq>(preq);
  auto rep = dynamic_pointer_cast<CacheRep>(prep);
  auto req_cnt = _req_cnt++;
  auto key = req->key();
  auto val = req->value();
  log(CACHESRV, "CacheSrv.Put req({}) key={}", req_cnt, key);
  // Take the lock
  std::lock_guard<std::mutex> guard(_mu);
  // If the shard isn't present, return an error
  if (!_cache.contains(req->shard())) {
    log(CACHESRV_ERR, "CacheSrv.Put rep({}) shard {} not found", req_cnt,
        req->shard());
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::Terror::TErrNotfound,
                             std::format("shard {}", req->shard())));
  }
  // Get the shard
  auto s = _cache.at(req->shard());
  s->Put(key, std::make_shared<std::string>(req->value()));
  log(CACHESRV, "CacheSrv.Put rep({}) latency={:0.3f}ms", req_cnt,
      LatencyMS(start));
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Init(int old_n_srv,
                                                   int new_n_srv) {
  auto start = GetCurrentTime();
  // Create shards
  for (uint32_t i = 0; i < NSHARD; i++) {
    _cache[i] = std::make_shared<Shard>();
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                  "CacheSrv make shards");
  // List of servers to steal shards from, and the map of shards to steal from
  // each server
  std::map<int, std::vector<uint32_t>> shards_to_steal;
  std::vector<int> src_srvs;
  for (int i = 0; i < old_n_srv; i++) {
    src_srvs.push_back(i);
    shards_to_steal[i] = std::vector<uint32_t>();
  }
  int nrpc = 0;
  std::vector<uint64_t> rpc_idxs;
  for (uint32_t i = 0; i < NSHARD; i++) {
    if (i % new_n_srv == _srv_id) {
      // If this server should host the shard in the new configuration, try to
      // steal it
      int src_srv = (int)i % old_n_srv;
      // Add this shard to the list of shards to steal from the source server
      shards_to_steal[src_srv].push_back(i);
      rpc_idxs.push_back(nrpc++);
    }
  }
  log(CACHESRV, "Load shard dumps from old servers nshard: {}", nrpc);
  auto startLoad = GetCurrentTime();
  if (!_sp_clnt->ProcEnv()->GetRunBootScript()) {
    // For each source server, dump shards to be stolen
    for (int src_srv : src_srvs) {
      for (uint32_t shard : shards_to_steal[src_srv]) {
        auto res = _cache_clnt->DumpShard(shard, false);
        if (!res.has_value()) {
          log(CACHESRV_ERR, "Error DumpShard: {}", res.error());
          fatal("Error DumpShard: {}", res.error().String());
          return std::unexpected(res.error());
        }
        log(CACHESRV, "Load shard {}", (int)shard);
        // Fill the local copy of the shard with the dumped values
        auto kvs = res.value();
        _cache.at(shard)->Fill(kvs);
      }
    }
  } else {
    {
      auto start = GetCurrentTime();
      auto res = _cache_clnt->BatchFetchDelegatedRPCs(rpc_idxs, 2 * rpc_idxs.size());
      if (!res.has_value()) {
        log(CACHESRV_ERR, "Error BatchFetchDelegatedRPCs: {}", res.error());
        fatal("Error BatchFetchDelegatedRPCs: {}", res.error().String());
        return std::unexpected(res.error());
      }
      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                      _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                      "Scaler.BatchFetchDelegatedRPCs");
    }
    auto start = GetCurrentTime();
    uint64_t rpc_idx = 0;
    // For each source server, dump shards to be stolen
    for (int src_srv : src_srvs) {
      for (uint32_t shard : shards_to_steal[src_srv]) {
        auto start = GetCurrentTime();
        auto res = _cache_clnt->DelegatedDumpShard(rpc_idx);
        if (!res.has_value()) {
          log(CACHESRV_ERR, "Error DelegatedDumpShard: {}", res.error());
          fatal("Error DelegatedDumpShard: {}", res.error().String());
          return std::unexpected(res.error());
        }
        LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                        _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                        "Scaler.DelegatedDumpRPC");
        log(CACHESRV, "Load shard delegated {}", (int)shard);
        start = GetCurrentTime();
        // Fill the local copy of the shard with the dumped values
        auto kvs = res.value();
        _cache.at(shard)->Fill(kvs);
        LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                        _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                        "Scaler.FillShard");
        rpc_idx++;
      }
    }
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                    "Scaler.DelegatedDumpRPCs");
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), startLoad,
                  "Scaler.LoadCacheState");
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), start, "CacheSrv.Init");
  return 0;
}

[[noreturn]] void Srv::Run() { _srv->Run(); }

};  // namespace apps::cache
};  // namespace sigmaos
