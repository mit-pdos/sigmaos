#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>

#include <util/log/log.h>
#include <io/net/srv.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <serr/serr.h>
#include <sigmap/sigmap.pb.h>
#include <sigmap/const.h>
#include <rpc/srv.h>
#include <proxy/sigmap/sigmap.h>
#include <apps/echo/proto/example_echo_server.pb.h>

namespace sigmaos {
namespace apps::echo {

const std::string ECHOSRV = "ECHOSRV";
const std::string ECHOSRV_ERR = "ECHOSRV" + sigmaos::util::log::ERR;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _sp_clnt(sp_clnt) {
    log(ECHOSRV, "Starting RPC srv");
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt);
    log(ECHOSRV, "Started RPC srv");
    auto echo_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("EchoSrv.Echo", std::make_shared<EchoReq>(), std::make_shared<EchoRep>(), std::bind(&Srv::Echo, this, std::placeholders::_1, std::placeholders::_2));
    _srv->RegisterRPCEndpoint(echo_ep);
    log(ECHOSRV, "Registered echo ep");
    {
      auto ep = _srv->GetEndpoint();
      auto res = sp_clnt->RegisterEP("name/echo-srv-cpp", ep);
      if (!res.has_value()) {
        log(ECHOSRV_ERR, "Error RegisterEP: {}", res.error());
        throw std::runtime_error(std::format("Error RegisterEP: {}", res.error().String()));
      }
      log(ECHOSRV, "Registered sigmaEP in realm namespace");
    }
  }
  ~Srv() {}

  [[noreturn]] void Run();

  private:
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::rpc::srv::Srv> _srv;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
  
  // Echo RPC handler
  std::expected<int, sigmaos::serr::Error> Echo(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
};


};
};
