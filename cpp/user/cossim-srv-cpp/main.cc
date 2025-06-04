#include <string>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <util/perf/perf.h>
#include <rpc/srv.h>
#include <rpc/spchannel/spchannel.h>
#include <sigmap/const.h>

#include <apps/cossim/srv.h>

int main(int argc, char *argv[]) {
  auto pe = sigmaos::proc::GetProcEnv();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), google::protobuf::util::TimeUtil::GetEpoch(), "E2e spawn time since spawn until main");
  LogSpawnLatency(pe->GetPID(), google::protobuf::util::TimeUtil::GetEpoch(), sigmaos::proc::GetExecTime(), "proc.exec_proc");
  sigmaos::util::log::init_logger(sigmaos::apps::cossim::COSSIMSRV);
  auto start = GetCurrentTime();
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start, "Create spproxyclnt");

  if (argc < 3) {
    fatal("Usage: {} N_VEC VEC_LEN", argv[0]);
  }

  std::string str_nvec = argv[1];
  int nvec = std::stoi(str_nvec);

  std::string str_vec_dim = argv[1];
  int vec_dim = std::stoi(str_vec_dim);

  // Create the echo server
  start = GetCurrentTime();
  auto srv = std::make_shared<sigmaos::apps::cossim::Srv>(sp_clnt, nvec, vec_dim);
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), start, "Make CosSim");
  // Run the server
  srv->Run();
}
