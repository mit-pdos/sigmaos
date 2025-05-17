#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <rpc/srv.h>
#include <sigmap/const.h>
#include <apps/echo/proto/example_echo_server.pb.h>

const std::string ECHOSRV = "ECHOSRV";

void wait_for_eviction(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) {
  log(ECHOSRV, "Waiting for eviction");
  auto res = sp_clnt->WaitEvict();
  if (!res.has_value()) {
    log(ECHOSRV, "Error WaitEvict: {}", res.error());
  }
  log(ECHOSRV, "Done waiting for eviction");
}

std::expected<int, sigmaos::serr::Error> Echo(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep) {
  auto req = dynamic_pointer_cast<EchoReq>(preq);
  auto rep = dynamic_pointer_cast<EchoRep>(prep);
  log(ECHOSRV, "Echo msg {}", req->text());
  rep->set_text(req->text());
  return 0;
}

int main(int argc, char *argv[]) {
  sigmaos::util::log::init_logger(ECHOSRV);
  log(ECHOSRV, "Running");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  std::thread evict_thread(wait_for_eviction, sp_clnt);

  log(ECHOSRV, "Starting sesssrv");
  auto srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt);
  log(ECHOSRV, "Sesssrv started");
  {
    auto ep = srv->GetEndpoint();
    auto res = sp_clnt->RegisterEP("name/echo-srv-cpp", ep);
    if (!res.has_value()) {
      log(ECHOSRV, "Error RegisterEP: {}", res.error());
    }
  }
  auto rpce = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("EchoSrv.Echo", std::make_shared<EchoReq>(), std::make_shared<EchoRep>(), Echo);
  srv->RegisterRPCEndpoint(rpce);
  log(ECHOSRV, "Echosrv registered ep");
  {
    auto res = sp_clnt->Started();
    if (!res.has_value()) {
      log(ECHOSRV, "Error Started: {}", res.error());
    }
  }
  log(ECHOSRV, "Server started");

  evict_thread.join();

  std::string msg("Evicted! Done serving.");
  sigmaos::proc::Tstatus exit_status = sigmaos::proc::Tstatus::StatusEvicted;

  {
    auto res = sp_clnt->Exited(exit_status, msg);
    if (!res.has_value()) {
      log(ECHOSRV, "Error exited: {}", res.error());
    }
  }
  std::exit(0);
}
