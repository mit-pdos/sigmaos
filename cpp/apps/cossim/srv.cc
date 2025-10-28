#include <apps/cossim/srv.h>

namespace sigmaos {
namespace apps::cossim {

bool Srv::_l = sigmaos::util::log::init_logger(COSSIMSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(COSSIMSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::CosSim(
    std::shared_ptr<google::protobuf::Message> preq,
    std::shared_ptr<google::protobuf::Message> prep) {
  // Register that a request was received
  _perf->TptTick(1.0);
  auto start = GetCurrentTime();
  auto req = dynamic_pointer_cast<CosSimReq>(preq);
  auto rep = dynamic_pointer_cast<CosSimRep>(prep);
  auto input_vec = std::make_shared<sigmaos::apps::cossim::Vector>(
      req->mutable_inputvec(), _vec_dim);
  auto input = req->inputvec().vals();
  auto const &v_ranges = req->vecranges();
  log(COSSIMSRV,
      "CosSim req({}) invec_sz={} n_ranges={} range[0].start_id={} "
      "range[0].end_id={}",
      req->id(), input.size(), v_ranges.size(), v_ranges[0].startid(),
      v_ranges[0].endid());
  double max = 0.0;
  uint64_t max_id = 0;
  for (auto const &vr : v_ranges) {
    for (int id = vr.startid(); id <= vr.endid(); id++) {
      {
        // Fetch the vector if it has not been fetched already
        auto res = fetch_vector(id);
        if (!res.has_value()) {
          log(COSSIMSRV_ERR, "Can't fetch vector {}", id);
          return res;
        }
      }
      auto vec = _vec_db.at(id);
      double cos_sim = input_vec->CosineSimilarity(vec);
      // Compare to max cosine similarity found so far
      if (cos_sim > max) {
        max_id = id;
        max = cos_sim;
      }
    }
  }
  rep->set_id(max_id);
  rep->set_val(max);
  if (!_first_req_ran.exchange(true)) {
    log(SPAWN_LAT, "First request ran");
  }
  log(COSSIMSRV,
      "CosSim rep({}) invec_sz={} max_id={} max={} latency={:0.3f}ms",
      req->id(), input.size(), max_id, max, LatencyMS(start));
  return 0;
}

void Srv::fetch_init_vectors_from_cache(
    std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>
        result,
    int srv_id, std::shared_ptr<std::vector<std::string>> key_vec,
    std::shared_ptr<std::vector<int>> key_vec_int) {
  int nbyte = 0;
  auto start = GetCurrentTime();
  std::shared_ptr<std::string> buf;
  std::vector<uint64_t> lengths;
  // If retrieving delegated initialization RPCs
  if (_sp_clnt->ProcEnv()->GetRunBootScript()) {
    std::shared_ptr<std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>
        vals;
    // Get the serialized vector from cached
    {
      auto res = _cache_clnt->DelegatedMultiGet(srv_id);
      if (!res.has_value()) {
        log(COSSIMSRV_ERR, "Error DelegatedMultiGet {}", res.error().String());
        result->set_value(std::unexpected(res.error()));
        return;
      }
      vals = res.value();
    }
    log(COSSIMSRV, "Got shards delegated RPC #{}", srv_id);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start, "GetShard RPC");
    // Sanity check
    if (key_vec_int->size() != vals->size()) {
      fatal("Key vec and returned vals ({}) don't match in size: {} != {}",
            srv_id, (int)key_vec_int->size(), (int)vals->size());
    }
    // Take the lock while modifying the _vec_db map
    std::lock_guard<std::mutex> guard(_mu);
    start = GetCurrentTime();
    uint64_t off = 0;
    for (int j = 0; j < key_vec_int->size(); j++) {
      int id = key_vec_int->at(j);
      _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(vals->at(j),
                                                                    _vec_dim);
      nbyte += vals->at(j)->GetStringView()->size();
    }
    log(COSSIMSRV, "Done parsing shard delegated RPC #{}", srv_id);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                    "Parse vecs & construct DB delegated");
  } else {
    std::shared_ptr<std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>
        vals;
    // Get the serialized vector from cached
    {
      auto res = _cache_clnt->MultiGet(srv_id, *key_vec);
      if (!res.has_value()) {
        log(COSSIMSRV_ERR, "Error MultiGet {}", res.error().String());
        result->set_value(std::unexpected(res.error()));
        return;
      }
      vals = res.value();
    }
    log(COSSIMSRV, "Got shards direct RPC");
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start, "GetShard RPC");
    // Take the lock while modifying the _vec_db map
    std::lock_guard<std::mutex> guard(_mu);
    start = GetCurrentTime();
    uint64_t off = 0;
    for (int j = 0; j < key_vec_int->size(); j++) {
      int id = key_vec_int->at(j);
      _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(vals->at(j),
                                                                    _vec_dim);
      nbyte += vals->at(j)->GetStringView()->size();
    }
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                    "Parse vecs & construct DB");
  }
  result->set_value(nbyte);
}

std::expected<int, sigmaos::serr::Error> Srv::Init() {
  std::map<uint32_t, std::shared_ptr<std::vector<std::string>>> key_vecs;
  std::map<uint32_t, std::shared_ptr<std::vector<int>>> key_vecs_int;
  for (uint32_t i = 0; i < _nvec; i++) {
    std::string i_str = std::to_string(i);
    uint32_t server_id = sigmaos::apps::cache::key2server(i_str, _ncache);
    if (!key_vecs.contains(server_id)) {
      key_vecs[server_id] = std::make_shared<std::vector<std::string>>();
      key_vecs_int[server_id] = std::make_shared<std::vector<int>>();
    }
    key_vecs[server_id]->push_back(i_str);
    key_vecs_int[server_id]->push_back(i);
  }
  // If not running boot script, pre-estables cached connections
  if (!_sp_clnt->ProcEnv()->GetRunBootScript()) {
    // Establish connections to cached servers
    auto startConnect = GetCurrentTime();
    {
      auto res = _cache_clnt->InitClnts(_ncache);
      if (!res.has_value()) {
        log(COSSIMSRV_ERR, "Error InitClnts: {}", res.error());
        fatal("Error InitClnts: {}", res.error().String());
        return std::unexpected(res.error());
      }
    }
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), startConnect,
                    "Initialization.ConnectionSetup");
  }
  int nbyte = 0;
  auto startLoad = GetCurrentTime();
  std::vector<std::thread> fetch_threads;
  std::vector<
      std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>>
      fetch_promises;
  std::vector<std::future<std::expected<int, sigmaos::serr::Error>>>
      fetch_results;
  // Start fetches in multiple threads
  for (int srv_id = 0; srv_id < _ncache; srv_id++) {
    fetch_promises.push_back(
        std::make_shared<
            std::promise<std::expected<int, sigmaos::serr::Error>>>());
    fetch_results.push_back(fetch_promises.at(srv_id)->get_future());
    fetch_threads.push_back(std::thread(
        &Srv::fetch_init_vectors_from_cache, this, fetch_promises.at(srv_id),
        srv_id, key_vecs.at(srv_id), key_vecs_int.at(srv_id)));
  }
  for (int i = 0; i < fetch_threads.size(); i++) {
    fetch_threads.at(i).join();
    auto res = fetch_results.at(i).get();
    if (!res.has_value()) {
      log(COSSIMSRV_ERR, "Error fetch_init_vectors_from_cache {}",
          res.error().String());
      return std::unexpected(res.error());
    }
    nbyte += res.value();
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), startLoad,
                  "Initialization.LoadState");
  LogSpawnLatency(
      _sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(),
      startLoad,
      std::format("Initialize soft state vector DB: {}B", (int)nbyte));
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::fetch_vector(uint64_t id) {
  // Lock to make sure fetching is atomic
  std::lock_guard<std::mutex> guard(_mu);
  // If map already contains key, bail out early
  if (_vec_db.contains(id)) {
    return 0;
  }
  auto start = GetCurrentTime();
  auto b = std::make_shared<std::string>();
  // Get the serialized vector from cached
  {
    auto res = _cache_clnt->Get(std::to_string(id), b);
    if (!res.has_value()) {
      return res;
    }
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Get vector");
  // Parse the vector
  auto start_parse_and_alloc = GetCurrentTime();
  _vec_db[id] =
      std::make_shared<sigmaos::apps::cossim::Vector>(b, b->data(), _vec_dim);
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), start_parse_and_alloc,
                  "Parse & alloc vector");
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                  _sp_clnt->ProcEnv()->GetSpawnTime(), start,
                  "Fetch vector e2e");
  return b->size();
}

[[noreturn]] void Srv::Run() { _srv->Run(); }

};  // namespace apps::cossim
};  // namespace sigmaos
