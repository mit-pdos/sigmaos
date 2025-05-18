#include <apps/echo/srv.h>

namespace sigmaos {
namespace apps::echo {

bool Srv::_l = sigmaos::util::log::init_logger(ECHOSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(ECHOSRV_ERR);

std::expected<int, sigmaos::serr::Error> Srv::Echo(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  auto req = dynamic_pointer_cast<EchoReq>(preq);
  auto rep = dynamic_pointer_cast<EchoRep>(prep);
  log(ECHOSRV, "Echo msg {}", req->text());
  rep->set_text(req->text());
  rep->set_res(req->num1() + req->num2());
  return 0;
}

[[noreturn]] void Srv::Run() {
  _srv->Run();
}

};
};
