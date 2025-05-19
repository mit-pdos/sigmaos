#include <apps/spin/srv.h>

namespace sigmaos {
namespace apps::spin {

bool Srv::_l = sigmaos::util::log::init_logger(SPINSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(SPINSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::Spin(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  auto req = dynamic_pointer_cast<SpinReq>(preq);
  auto rep = dynamic_pointer_cast<SpinRep>(prep);
  int64_t n = req->n();
  log(SPINSRV, "Spin n={}", n);
  int64_t x = 1;
  for (int64_t i = 1; i < n; i++) {
    x = (x * (int64_t) i) + 1;
  }
  rep->set_n(x);
  log(SPINSRV, "Spin n={} done", req->n());
  return 0;
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
