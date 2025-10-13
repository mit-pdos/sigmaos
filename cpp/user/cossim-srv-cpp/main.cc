#include <apps/cossim/srv.h>
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
  LogRuntimeInitLatency(pe->GetPID(), pe->GetSpawnTime());
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(),
                  google::protobuf::util::TimeUtil::GetEpoch(),
                  "E2e spawn time since spawn until main");
  LogSpawnLatency(pe->GetPID(), google::protobuf::util::TimeUtil::GetEpoch(),
                  sigmaos::proc::GetExecTime(), "proc.exec_proc");
  sigmaos::util::log::init_logger(sigmaos::apps::cossim::COSSIMSRV);
  auto start = GetCurrentTime();
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start,
                  "Create spproxyclnt");

  if (argc < 6) {
    fatal("Usage: {} CACHE_PN N_CACHE N_VEC VEC_LEN EAGER_INIT", argv[0]);
  }

  std::string cache_pn = argv[1];

  std::string str_ncache = argv[2];
  int ncache = std::stoi(str_ncache);

  std::string str_nvec = argv[3];
  int nvec = std::stoi(str_nvec);

  std::string str_vec_dim = argv[4];
  int vec_dim = std::stoi(str_vec_dim);

  std::string str_eager_init = argv[5];
  bool eager_init = str_eager_init == "true";

  // Create the echo server
  start = GetCurrentTime();
  auto srv = std::make_shared<sigmaos::apps::cossim::Srv>(
      sp_clnt, nvec, vec_dim, cache_pn, ncache, eager_init);
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start, "Make CosSim");
  // Run the server
  srv->Run();
}
