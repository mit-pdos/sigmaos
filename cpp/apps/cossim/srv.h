#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>
#include <limits>
#include <cmath>

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
#include <apps/cache/clnt.h>
#include <apps/cossim/proto/cossim.pb.h>

namespace sigmaos {
namespace apps::cossim {

const std::string COSSIMSRV = "COSSIMSRV";
const std::string COSSIMSRV_ERR = "COSSIMSRV" + sigmaos::util::log::ERR;

const std::filesystem::path COSSIM_SVC_NAME = "name/cossim";
const int INIT_NTHREAD = 100;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt, int nvec, int vec_dim, std::string cache_clnt_pn) : _nvec(nvec), _vec_dim(vec_dim), _vec_db(), _sp_clnt(sp_clnt), _cache_clnt(std::make_shared<sigmaos::apps::cache::Clnt>(sp_clnt, cache_clnt_pn)) {
    log(COSSIMSRV, "Starting RPC srv");
    auto start = GetCurrentTime();
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt, INIT_NTHREAD);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Make RPCSrv");
    log(COSSIMSRV, "Started RPC srv");
    auto cossim_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("CosSimSrv.CosSim", std::make_shared<CosSimReq>(), std::make_shared<CosSimRep>(), std::bind(&Srv::CosSim, this, std::placeholders::_1, std::placeholders::_2));
    _srv->ExposeRPCHandler(cossim_ep);
    log(COSSIMSRV, "Exposed cossim RPC handler");
    log(COSSIMSRV, "Init cache clnt");
    {
      auto res = _cache_clnt->Init();
      if (!res.has_value()) {
        fatal("Error Init cacheclnt: {}", res.error().String());
      }
    }
    log(COSSIMSRV, "Done init cache clnt");
    log(COSSIMSRV, "Init srv");
    {
      auto res = Init();
      if (!res.has_value()) {
        fatal("Error Init: {}", res.error().String());
      }
    }
    log(COSSIMSRV, "Done init srv");
    {
      auto pn = COSSIM_SVC_NAME;
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
  std::expected<int, sigmaos::serr::Error> Init();
  [[noreturn]] void Run();

  private:
  int _nvec;
  int _vec_dim;
  std::map<uint64_t, std::vector<double>> _vec_db;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::apps::cache::Clnt> _cache_clnt;
  std::shared_ptr<sigmaos::rpc::srv::Srv> _srv;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
  
  // CosSim RPC handler
  std::expected<int, sigmaos::serr::Error> CosSim(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
};


};
};
