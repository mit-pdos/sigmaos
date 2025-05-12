#include <session/srv/srv.h>

namespace sigmaos {
namespace session::srv {

bool Srv::_l = sigmaos::util::log::init_logger(SESSSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(SESSSRV_ERR);

//  log(ECHOSRV, "Starting net server");
//  auto srv = std::make_shared<sigmaos::io::net::Srv>();
//  int port = srv->GetPort();
//  log(ECHOSRV, "Net server started with port {}", port);
//  {
//    auto ep = std::make_shared<TendpointProto>();
//    auto addr = ep->add_addr();
//    addr->set_ipstr("127.0.0.1");
//    addr->set_portint(port);
//    ep->set_type(sigmaos::sigmap::constants::EXTERNAL_EP);
//    auto res = sp_clnt->RegisterEP("name/echo-srv-cpp", ep);
//    if (!res.has_value()) {
//      log(ECHOSRV, "Error RegisterEP: {}", res.error());
//    }
//  }
//


std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Srv::serve_request(std::shared_ptr<sigmaos::io::transport::Call> req) {
  log(SESSSRV, "session::Srv::serve_request");
  throw std::runtime_error("unimplemented");
}

std::shared_ptr<TendpointProto> Srv::GetEndpoint() {
  auto ep = std::make_shared<TendpointProto>();
  auto addr = ep->add_addr();
  // TODO: other IP addresses?
  addr->set_ipstr("127.0.0.1");
  addr->set_portint(_netsrv->GetPort());
  ep->set_type(sigmaos::sigmap::constants::EXTERNAL_EP);
  return ep;
}


};
};
