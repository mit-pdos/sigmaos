#include <rpc/srv.h>

namespace sigmaos {
namespace rpc::srv {

bool Srv::_l = sigmaos::util::log::init_logger(RPCSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(RPCSRV_ERR);

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> Srv::serve_request(std::shared_ptr<sigmaos::io::transport::Call> req) {
  log(RPCSRV, "session::Srv::serve_request");
  throw std::runtime_error("unimplemented");
}

std::shared_ptr<TendpointProto> Srv::GetEndpoint() {
  auto ep = std::make_shared<TendpointProto>();
  auto addr = ep->add_addr();
  // TODO: other IP addresses?
  addr->set_ipstr(_sp_clnt->ProcEnv()->GetOuterContainerIP());
  addr->set_portint(_netsrv->GetPort());
  ep->set_type(sigmaos::sigmap::constants::EXTERNAL_EP);
  return ep;
}

};
};
