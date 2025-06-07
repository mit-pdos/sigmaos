#include <apps/cossim/srv.h>

namespace sigmaos {
namespace apps::cossim {

bool Srv::_l = sigmaos::util::log::init_logger(COSSIMSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(COSSIMSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::CosSim(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  auto req = dynamic_pointer_cast<CosSimReq>(preq);
  auto rep = dynamic_pointer_cast<CosSimRep>(prep);
  auto input = req->inputvec().vals();
  auto const &v_ranges = req->vecranges();
  log(COSSIMSRV, "CosSim invec={}", input.size());
  double max = 0.0;
  uint64_t max_id = 0;
  for (auto const &vr : v_ranges) {
    for (int id = vr.startid(); id <= vr.endid(); id++) {
      auto vec = _vec_db.at(id);
      // Compute cosine similarity
      double input_l2 = 0.0;
      double vec_l2 = 0.0;
      double cos_sim = 0.0;
      for (int i = 0; i < input.size(); i++) {
        cos_sim += input[i] * vec[i];
        input_l2 += input[i] * input[i];
        vec_l2 += vec[i] * vec[i];
      }
      cos_sim /= (std::sqrt(input_l2) * std::sqrt(vec_l2));
      // Compare to max cosine similarity found so far
      if (cos_sim > max) {
        max_id = id;
        max = cos_sim;
      }
    }
  }
  rep->set_id(max_id);
  rep->set_val(max);
  log(COSSIMSRV, "CosSim invec={} max_id={} max={}", input.size(), max_id, max);
  return 0;
}

std::expected<int, sigmaos::serr::Error> Srv::Init() {
  for (int i = 0; i < _nvec; i++) {
    std::string b(10000, '\0');
    // Get the serialized vector from cached
    {
      auto res = _cache_clnt->Get(std::to_string(i), &b);
      if (!res.has_value()) {
        return res;
      }
    }
    // Parse the vector
    Vector v;
    v.ParseFromString(b);
    _vec_db[i] = std::vector<double>(v.vals().begin(), v.vals().end());
  }
  return 0;
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
