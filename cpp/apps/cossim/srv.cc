#include <apps/cossim/srv.h>

namespace sigmaos {
namespace apps::cossim {

bool Srv::_l = sigmaos::util::log::init_logger(COSSIMSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(COSSIMSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::CosSim(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  auto req = dynamic_pointer_cast<CosSimReq>(preq);
  auto rep = dynamic_pointer_cast<CosSimRep>(prep);
  std::string input = req->inputvec();
  int64_t n = req->n();
  log(COSSIMSRV, "CosSim invec={} n={}", input.size(), n);
  int64_t x = 1;
  for (int64_t i = 1; i < n; i++) {
    x = (x * (int64_t) i) + 1;
  }
  rep->set_id(x);
  log(COSSIMSRV, "CosSim invec={} n={}", input.size(), n);
  return 0;
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
