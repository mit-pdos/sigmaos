#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>

#include <util/log/log.h>
#include <util/perf/perf.h>
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
#include <apps/cossim/proto/cossim.pb.h>

namespace sigmaos {
namespace apps::cossim {

const std::string COSSIMSRV = "COSSIMSRV";
const std::string COSSIMSRV_ERR = "COSSIMSRV" + sigmaos::util::log::ERR;

const std::filesystem::path COSSIMSRV_UNION_DIR_PN = "name/cossim";
const int INIT_NTHREAD = 100;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _sp_clnt(sp_clnt){
    log(COSSIMSRV, "Starting RPC srv");
    auto start = GetCurrentTime();
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt, INIT_NTHREAD);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Make RPCSrv");
    log(COSSIMSRV, "Started RPC srv");
    auto cossim_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("CosSimSrv.CosSim", std::make_shared<CosSimReq>(), std::make_shared<CosSimRep>(), std::bind(&Srv::CosSim, this, std::placeholders::_1, std::placeholders::_2));
    _srv->ExposeRPCHandler(cossim_ep);
    log(COSSIMSRV, "Exposed cossim RPC handler");
    {
      auto pn = COSSIMSRV_UNION_DIR_PN / std::filesystem::path(_sp_clnt->ProcEnv()->GetPID());
      auto start = GetCurrentTime();
      auto res = _srv->RegisterEP(pn);
      if (!res.has_value()) {
        log(COSSIMSRV_ERR, "Error RegisterEP: {}", res.error());
        fatal("Error RegisterEP: {}", res.error().String());
      }
      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "RegisterEP");
      log(COSSIMSRV, "Registered sigmaEP");
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
  
  // CosSim RPC handler
  std::expected<int, sigmaos::serr::Error> CosSim(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
};


};
};
