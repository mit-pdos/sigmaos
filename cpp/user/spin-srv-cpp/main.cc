#include <iostream>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <util/perf/perf.h>
#include <rpc/srv.h>
#include <sigmap/const.h>

#include <apps/spin/srv.h>

int main(int argc, char *argv[]) {
  auto pe = sigmaos::proc::GetProcEnv();
  LogSpawnLatency(pe->GetPID(), pe->GetSpawnTime(), google::protobuf::util::TimeUtil::GetEpoch(), "E2e spawn time since spawn until main");
  LogSpawnLatency(pe->GetPID(), google::protobuf::util::TimeUtil::GetEpoch(), sigmaos::proc::GetExecTime(), "proc.exec_proc");
  auto t = google::protobuf::util::TimeUtil::GetCurrentTime();
  sigmaos::util::log::init_logger(sigmaos::apps::spin::SPINSRV);
  log(sigmaos::apps::spin::SPINSRV, "main");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  // Create the echo server
  auto srv = std::make_shared<sigmaos::apps::spin::Srv>(sp_clnt);
  // Run the server
  srv->Run();
}
