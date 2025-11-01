#pragma once

#include <apps/cache/cache.h>
#include <apps/cache/clnt.h>
#include <apps/cache/shard.h>
#include <io/conn/conn.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <io/net/srv.h>
#include <io/transport/transport.h>
#include <proxy/sigmap/sigmap.h>
#include <rpc/srv.h>
#include <serr/serr.h>
#include <sigmap/const.h>
#include <sigmap/sigmap.pb.h>
#include <util/log/log.h>
#include <util/perf/perf.h>

#include <cmath>
#include <expected>
#include <filesystem>
#include <format>
#include <future>
#include <limits>
#include <memory>
#include <vector>

namespace sigmaos {
namespace apps::cache {

const std::string CACHESRV = "CACHESRV";
const std::string CACHESRV_ERR = "CACHESRV" + sigmaos::util::log::ERR;

const int INIT_NTHREAD = 100;

class Srv {
 public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt,
      std::string cache_dir, std::string job_name, std::string srv_pn,
      bool use_ep_cache, int old_n_srv, int new_n_srv, int srv_id,
      bool migrated)
      : _mu(),
        _srv_id(srv_id),
        _migrated(migrated),
        _cache_dir(cache_dir),
        _req_cnt(0),
        _cache(),
        _sp_clnt(sp_clnt),
        _perf(std::make_shared<sigmaos::util::perf::Perf>(sp_clnt->ProcEnv(),
                                                          CACHESRV)),
        _cache_clnt(std::make_shared<sigmaos::apps::cache::Clnt>(
            sp_clnt, cache_dir, (uint32_t)old_n_srv)) {
    log(CACHESRV,
        "Starting RPC srv id:{} migrated:{} cachedir:{} jobname:{} srvpn:{} "
        "useEPCache:{} "
        "oldNSrv:{} newNSrv:{}",
        srv_id, migrated, cache_dir, job_name, srv_pn, use_ep_cache, old_n_srv,
        new_n_srv);
    auto start = GetCurrentTime();
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt, INIT_NTHREAD);
    LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                    _sp_clnt->ProcEnv()->GetSpawnTime(), start, "Make RPCSrv");
    auto get_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>(
        "CacheSrv.Get", std::make_shared<CacheReq>(),
        std::make_shared<CacheRep>(),
        std::bind(&Srv::Get, this, std::placeholders::_1,
                  std::placeholders::_2));
    _srv->ExposeRPCHandler(get_ep);
    auto put_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>(
        "CacheSrv.Put", std::make_shared<CacheReq>(),
        std::make_shared<CacheRep>(),
        std::bind(&Srv::Put, this, std::placeholders::_1,
                  std::placeholders::_2));
    _srv->ExposeRPCHandler(put_ep);
    log(CACHESRV, "Exposed cachesrv RPC handlers");
    {
      auto res = Init(old_n_srv, new_n_srv);
      if (!res.has_value()) {
        log(CACHESRV_ERR, "Error Init: {}", res.error());
        fatal("Error Init: {}", res.error().String());
      }
    }
    {
      std::string pn = cache_dir + "/" + srv_pn;
      auto start = GetCurrentTime();
      auto res = _srv->RegisterEP(pn);
      if (!res.has_value()) {
        log(CACHESRV_ERR, "Error RegisterEP: {}", res.error());
        fatal("Error RegisterEP: {}", res.error().String());
      }
      LogSpawnLatency(_sp_clnt->ProcEnv()->GetPID(),
                      _sp_clnt->ProcEnv()->GetSpawnTime(), start, "RegisterEP");
      log(CACHESRV, "Registered sigmaEP");
    }
    // Register performance tracker with RPCSrv infrastructure
    _srv->RegisterPerfTracker(_perf);
  }
  ~Srv() {}
  std::expected<int, sigmaos::serr::Error> Init(int old_n_srv, int new_n_srv);
  [[noreturn]] void Run();

 private:
  std::mutex _mu;
  int _srv_id;
  bool _migrated;
  std::string _cache_dir;
  std::atomic<uint64_t> _req_cnt;
  std::map<uint32_t, std::shared_ptr<Shard>> _cache;
  std::shared_ptr<sigmaos::apps::cache::Clnt> _cache_clnt;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::util::perf::Perf> _perf;
  std::atomic<bool> _first_req_ran;
  std::shared_ptr<sigmaos::rpc::srv::Srv> _srv;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<int, sigmaos::serr::Error> Get(
      std::shared_ptr<google::protobuf::Message> preq,
      std::shared_ptr<google::protobuf::Message> prep);
  std::expected<int, sigmaos::serr::Error> Put(
      std::shared_ptr<google::protobuf::Message> preq,
      std::shared_ptr<google::protobuf::Message> prep);
};

};  // namespace apps::cache
};  // namespace sigmaos
