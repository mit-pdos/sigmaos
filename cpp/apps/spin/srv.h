#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>

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
#include <apps/spin/proto/spin.pb.h>

namespace sigmaos {
namespace apps::spin {

const std::string SPINSRV = "SPINSRV";
const std::string SPINSRV_ERR = "SPINSRV" + sigmaos::util::log::ERR;

const std::filesystem::path SPINSRV_UNION_DIR_PN = "name/spin-srv-cpp";
const int INIT_NTHREAD = 100;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _sp_clnt(sp_clnt) {
    log(SPINSRV, "Starting RPC srv");
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt, INIT_NTHREAD);
    log(SPINSRV, "Started RPC srv");
    auto spin_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("SpinSrv.Spin", std::make_shared<SpinReq>(), std::make_shared<SpinRep>(), std::bind(&Srv::Spin, this, std::placeholders::_1, std::placeholders::_2));
    _srv->ExposeRPCHandler(spin_ep);
    log(SPINSRV, "Exposed spin RPC handler");
    {
      auto pn = SPINSRV_UNION_DIR_PN / std::filesystem::path(_sp_clnt->ProcEnv()->GetPID());
      auto res = _srv->RegisterEP(pn);
      if (!res.has_value()) {
        log(SPINSRV_ERR, "Error RegisterEP: {}", res.error());
        fatal("Error RegisterEP: {}", res.error().String());
      }
      log(SPINSRV, "Registered sigmaEP");
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
  
  // Spin RPC handler
  std::expected<int, sigmaos::serr::Error> Spin(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
};


};
};
