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
    std::vector<std::string> key_vec;
    std::shared_ptr<std::string> buf;
    std::vector<uint64_t> lengths;
    auto start = GetCurrentTime();
    log(COSSIMSRV, "Going to get shard");
    for (int i = 0; i < _nvec; i++) {
      key_vec.push_back(std::to_string(i));
    }
    // Get the serialized vector from cached
    {
      auto res = _cache_clnt->MultiGet(key_vec);
      if (!res.has_value()) {
        log(SPAWN_LAT, "Error get all-vecs {}", res.error().String());
        return std::unexpected(res.error());
      }
      auto res_pair = res.value();
      lengths = res_pair.first;
      buf = res_pair.second; 
    }
    log(COSSIMSRV, "Got shard");
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "GetShard RPC");
    start = GetCurrentTime();
    uint64_t off = 0;
    for (int id = 0; id < _nvec; id++) {
      log(COSSIMSRV, "parse vec {}", id);
      _vec_db[id] = std::make_shared<sigmaos::apps::cossim::Vector>(buf, buf->data() + off, _vec_dim);
      log(COSSIMSRV, "done parse vec {}", id);
      off += lengths[id];
    }
    log(COSSIMSRV, "done parsing all vecs");
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Parse vecs & construct DB");
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
