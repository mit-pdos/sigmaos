#include <apps/cache/srv.h>

namespace sigmaos {
namespace apps::cache {

bool Srv::_l = sigmaos::util::log::init_logger(CACHESRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(CACHESRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::Get(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
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
    log(CACHESRV_ERR, "CacheSrv.Get rep({}) shard {} not found", req_cnt, req->shard());
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrNotfound, std::format("shard {}", req->shard())));
  }
  std::shared_ptr<std::string> val;
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
  log(CACHESRV, "CacheSrv.Get rep({}) latency={:0.3f}ms", req_cnt, LatencyMS(start));
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Put(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  // Register that a request was received
  _perf->TptTick(1.0);
  auto start = GetCurrentTime();
  auto req = dynamic_pointer_cast<CacheReq>(preq);
  auto rep = dynamic_pointer_cast<CacheRep>(prep);
  auto req_cnt = _req_cnt++;
  auto key = req->key();
  auto val = req->value();
//  auto input_vec = std::make_shared<sigmaos::apps::cossim::Vector>(req->mutable_inputvec(), _vec_dim);
//  auto input = req->inputvec().vals();
//  auto const &v_ranges = req->vecranges();
  log(CACHESRV, "CacheSrv.Put req({}) key={}", req_cnt, key);
  // TODO: put into map
  // TODO: handle misses
  log(CACHESRV, "CacheSrv.Put rep({}) latency={:0.3f}ms", req_cnt, LatencyMS(start));
  fatal("unimplemented");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Init() {
  fatal("Unimplemented");
  return 0;
//  std::map<uint32_t, std::shared_ptr<std::vector<std::string>>> key_vecs;
//  std::map<uint32_t, std::shared_ptr<std::vector<int>>> key_vecs_int;
//  for (uint32_t i = 0; i < _nvec; i++) {
//    std::string i_str = std::to_string(i);
//    uint32_t server_id = sigmaos::apps::cache::key2server(i_str, _ncache);
//    if (!key_vecs.contains(server_id)) {
//      key_vecs[server_id] = std::make_shared<std::vector<std::string>>();
//      key_vecs_int[server_id] = std::make_shared<std::vector<int>>();
//    }
//    key_vecs[server_id]->push_back(i_str);
//    key_vecs_int[server_id]->push_back(i);
//  }
//  int nbyte = 0;
//  auto start = GetCurrentTime();
//  std::vector<std::thread> fetch_threads;
//  std::vector<std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>> fetch_promises;
//  std::vector<std::future<std::expected<int, sigmaos::serr::Error>>> fetch_results;
//  // Start fetches in multiple threads
//  for (int srv_id = 0; srv_id < _ncache; srv_id++) {
//    fetch_promises.push_back(std::make_shared<std::promise<std::expected<int, sigmaos::serr::Error>>>());
//    fetch_results.push_back(fetch_promises.at(srv_id)->get_future());
//    fetch_threads.push_back(std::thread(&Srv::fetch_init_vectors_from_cache, this, fetch_promises.at(srv_id), srv_id, key_vecs.at(srv_id), key_vecs_int.at(srv_id)));
//  }
//  for (int i = 0; i < fetch_threads.size(); i++) {
//    fetch_threads.at(i).join();
//    auto res = fetch_results.at(i).get();
//    if (!res.has_value()) {
//      log(COSSIMSRV_ERR, "Error fetch_init_vectors_from_cache {}", res.error().String());
//      return std::unexpected(res.error());
//    }
//    nbyte += res.value();
//  }
//  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, std::format("Initialize soft state vector DB: {}B", (int) nbyte));
//  return 0;
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
