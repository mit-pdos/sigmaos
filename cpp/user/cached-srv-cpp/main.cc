#include <apps/cache/srv.h>
#include <google/protobuf/util/time_util.h>
#include <proc/proc.h>
#include <proxy/sigmap/sigmap.h>
#include <rpc/spchannel/spchannel.h>
#include <rpc/srv.h>
#include <serr/serr.h>
#include <sigmap/const.h>
#include <util/log/log.h>
#include <util/perf/perf.h>

#include <string>

int main(int argc, char *argv[]) {
  auto pe = sigmaos::proc::GetProcEnv();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(),
                  google::protobuf::util::TimeUtil::GetEpoch(),
                  "E2e spawn time since spawn until main");
  LogSpawnLatency(pe->GetPID(), google::protobuf::util::TimeUtil::GetEpoch(),
                  sigmaos::proc::GetExecTime(), "proc.exec_proc");
  sigmaos::util::log::init_logger(sigmaos::apps::cache::CACHESRV);
  auto start = GetCurrentTime();
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start,
                  "Create spproxyclnt");

  if (argc < 7) {
    fatal("Usage: {} cachedir jobname srvpn useEPCache oldNSrv newNSrv",
          argv[0]);
  }

  std::string cache_dir = argv[1];
  std::string job_name = argv[2];
  std::string srv_pn = argv[3];
  std::string str_use_ep_cache = argv[4];
  bool use_ep_cache = str_use_ep_cache == "true";
  int old_n_srv = std::stoi(argv[5]);
  int new_n_srv = std::stoi(argv[6]);

  start = GetCurrentTime();
  auto srv = std::make_shared<sigmaos::apps::cache::Srv>(
      sp_clnt, cache_dir, job_name, srv_pn, use_ep_cache, old_n_srv, new_n_srv,
      new_n_srv);
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start, "Make CacheSrv");
  srv->Run();
}
