#include <apps/cossim/srv.h>

namespace sigmaos {
namespace apps::cossim {

bool Srv::_l = sigmaos::util::log::init_logger(COSSIMSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(COSSIMSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::CosSim(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  // Register that a request was received
  _perf->TptTick(1.0);
  auto start = GetCurrentTime();
  auto req = dynamic_pointer_cast<CosSimReq>(preq);
  auto rep = dynamic_pointer_cast<CosSimRep>(prep);
  auto input_vec = std::make_shared<sigmaos::apps::cossim::Vector>(req->mutable_inputvec(), _vec_dim);
  auto input = req->inputvec().vals();
  auto const &v_ranges = req->vecranges();
  log(COSSIMSRV, "CosSim req({}) invec={}", req->id(), input.size());
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
  log(COSSIMSRV, "CosSim rep({}) invec={} max_id={} max={} latency={:0.3f}ms", req->id(), input.size(), max_id, max, LatencyMS(start));
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Init() {
  int nbyte = 0;
  auto start = GetCurrentTime();
  if (false) {
    for (int i = 0; i < _nvec; i++) {
      auto res = fetch_vector(i);
      if (!res.has_value()) {
        return res;
      }
      nbyte += res.value();
    }
  } else {
    std::map<uint32_t, std::vector<std::string>> key_vecs;
    std::map<uint32_t, std::vector<int>> key_vecs_int;
    for (uint32_t i = 0; i < _nvec; i++) {
      std::string i_str = std::to_string(i);
      uint32_t server_id = sigmaos::apps::cache::key2server(i_str, _ncache);
      if (!key_vecs.contains(server_id)) {
        key_vecs[server_id] = std::vector<std::string>();
        key_vecs_int[server_id] = std::vector<int>();
      }
      key_vecs[server_id].push_back(i_str);
      key_vecs_int[server_id].push_back(i);
    }
    std::shared_ptr<std::string> buf;
    std::vector<uint64_t> lengths;
    auto start = GetCurrentTime();
    log(COSSIMSRV, "Going to get shard");
    // If retrieving delegated initialization RPCs
    if (_sp_clnt->ProcEnv()->GetDelegateInit()) {
      // Retrieve the RPC result for each cache server
      for (int i = 0; i < _ncache; i++) {
        // Get the serialized vector from cached
        {
          auto res = _cache_clnt->DelegatedMultiGet(i);
          if (!res.has_value()) {
            log(COSSIMSRV_ERR, "Error DelegatedMultiVec {}", res.error().String());
            return std::unexpected(res.error());
          }
          auto res_pair = res.value();
          lengths = res_pair.first;
          buf = res_pair.second; 
        }
        log(COSSIMSRV, "Got shards delegated RPC #{}", i);
        LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "GetShard RPC");
        start = GetCurrentTime();
        uint64_t off = 0;
        for (int j = 0; j < key_vecs_int.at(i).size(); j++) {
          int id = key_vecs_int.at(i).at(j);
          log(COSSIMSRV, "parse vec {}", id);
          _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(buf, buf->data() + off, _vec_dim);
          log(COSSIMSRV, "done parse vec {}", id);
          off += lengths[j];
          nbyte += lengths[j];
        }
        log(COSSIMSRV, "Done parsing shard delegated RPC #{}", i);
      }
      log(COSSIMSRV, "Parsed all vec shards from delegated RPCs & constructed DB");
      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Parse vecs & construct DB");
    } else {
      for (uint32_t i = 0; i < _ncache; i++) {
        // Get the serialized vector from cached
        {
          auto res = _cache_clnt->MultiGet(i, key_vecs[i]);
          if (!res.has_value()) {
            log(COSSIMSRV_ERR, "Error MultiGet {}", res.error().String());
            return std::unexpected(res.error());
          }
          auto res_pair = res.value();
          lengths = res_pair.first;
          buf = res_pair.second; 
        }
        log(COSSIMSRV, "Got shards direct RPC");
        LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "GetShard RPC");
        start = GetCurrentTime();
        uint64_t off = 0;
        for (int j = 0; j < key_vecs_int.at(i).size(); j++) {
          int id = key_vecs_int.at(i).at(j);
          log(COSSIMSRV, "parse vec {}", id);
          _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(buf, buf->data() + off, _vec_dim);
          log(COSSIMSRV, "done parse vec {}", id);
          off += lengths[j];
          nbyte += lengths[j];
        }
      }
      log(COSSIMSRV, "Parsed all vec shards from direct RPCs & constructed DB");
      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Parse vecs & construct DB");
    }
  }
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, std::format("Init soft state vector DB: {}B", (int) nbyte));
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
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Get vector");
  // Parse the vector
  auto start_parse_and_alloc = GetCurrentTime();
  _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(b, b->data(), _vec_dim);
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start_parse_and_alloc, "Parse & alloc vector");
  LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Fetch vector e2e");
  return b->size();
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
