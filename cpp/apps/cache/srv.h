#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>
#include <limits>
#include <future>
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
#include <apps/cache/shard.h>

namespace sigmaos {
namespace apps::cache {

const std::string CACHESRV = "CACHESRV";
const std::string CACHESRV_ERR = "CACHESRV" + sigmaos::util::log::ERR;

const int INIT_NTHREAD = 100;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt, std::string cache_dir, std::string job_name, std::string srv_pn, bool use_ep_cache, int old_n_srv, int new_n_srv) : _mu(),
    _req_cnt(0),
    _cache(),
    _sp_clnt(sp_clnt),
    _perf(std::make_shared<sigmaos::util::perf::Perf>(sp_clnt->ProcEnv(), CACHESRV)) {
    //_cache_clnt(std::make_shared<sigmaos::apps::cache::Clnt>(sp_clnt, cache_clnt_pn, (uint32_t) ncache)) {
    log(CACHESRV, "Starting RPC srv cachedir:{} jobname:{} srvpn:{} useEPCache:{} oldNSrv:{} newNSrv:{}", cache_dir, job_name, srv_pn, use_ep_cache, old_n_srv, new_n_srv);
    auto start = GetCurrentTime();
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt, INIT_NTHREAD);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Make RPCSrv"); 
    // TODO: register EPs
//    auto cached_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>("CacheSrv.CosSim", std::make_shared<CosSimReq>(), std::make_shared<CosSimRep>(), std::bind(&Srv::CosSim, this, std::placeholders::_1, std::placeholders::_2));
//    _srv->ExposeRPCHandler(cossim_ep);
    log(CACHESRV, "Exposed cachesrv RPC handlers");
    // TODO: Register EP
//    {
//      auto start = GetCurrentTime();
//      auto res = _srv->RegisterEP(pn);
//      if (!res.has_value()) {
//        log(CACHESRV_ERR, "Error RegisterEP: {}", res.error());
//        fatal("Error RegisterEP: {}", res.error().String());
//      }
//      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(), _sp_clnt->ProcEnv()->GetSpawnTime(), start, "RegisterEP");
//      log(CACHESRV, "Registered sigmaEP");
//    }
    // Register performance tracker with RPCSrv infrastructure
    _srv->RegisterPerfTracker(_perf);
    fatal("Unimplemented");
  }
  ~Srv() {}
  std::expected<int, sigmaos::serr::Error> Init();
  [[noreturn]] void Run();

  private:
  std::mutex _mu;
  std::atomic<uint64_t> _req_cnt;
  // TODO: shards & structs
  std::map<uint32_t, std::shared_ptr<Shard>> _cache;
//  std::shared_ptr<sigmaos::apps::cache::Clnt> _cache_clnt;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::util::perf::Perf> _perf;
  std::shared_ptr<sigmaos::rpc::srv::Srv> _srv;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
  
  std::expected<int, sigmaos::serr::Error> Get(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
  std::expected<int, sigmaos::serr::Error> Put(std::shared_ptr<google::protobuf::Message> preq, std::shared_ptr<google::protobuf::Message> prep);
};


};
};
