#include <iostream>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <util/perf/perf.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>

const std::string CPP_USER_PROC = "CPP_USER_PROC";

int main(int argc, char *argv[]) {
  auto t = google::protobuf::util::TimeUtil::GetCurrentTime();
  sigmaos::util::log::init_logger(CPP_USER_PROC);
  log(CPP_USER_PROC, "Running");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
  auto spawn_time = sp_clnt->ProcEnv()->GetSpawnTime();
  LogSpawnLatency(sp_clnt->ProcEnv()->GetPID(), spawn_time, google::protobuf::util::TimeUtil::GetEpoch(), "Time since spawn until main");
  {
    auto res = sp_clnt->Started();
    if (!res.has_value()) {
      log(CPP_USER_PROC, "Error started: {}", res.error());
    }
  }
  auto evict_thread = sp_clnt->StartWaitEvictThread();
  // Test connection to spproxyd
  log(CPP_USER_PROC, "Test");
  sp_clnt->Test();
  log(CPP_USER_PROC, "Done testing");

  std::string msg("Exited normally!");
  sigmaos::proc::Tstatus exit_status = sigmaos::proc::Tstatus::StatusOK;
  // Possibly wait for eviction
  if (argc > 1 && std::string(argv[1]) == "waitEvict") {
    evict_thread.join();
    msg = "Evicted!";
    exit_status = sigmaos::proc::Tstatus::StatusEvicted;
  }

  {
    auto res = sp_clnt->Exited(exit_status, msg);
    if (!res.has_value()) {
      log(CPP_USER_PROC, "Error exited: {}", res.error());
    }
  }
  std::exit(0);
}
