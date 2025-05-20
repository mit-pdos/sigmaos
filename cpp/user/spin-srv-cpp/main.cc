#include <iostream>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <rpc/srv.h>
#include <sigmap/const.h>

#include <apps/spin/srv.h>

int main(int argc, char *argv[]) {
  auto t = google::protobuf::util::TimeUtil::GetCurrentTime();
  sigmaos::util::log::init_logger(sigmaos::apps::spin::SPINSRV);
  log(sigmaos::apps::spin::SPINSRV, "main");
  log(SPAWN_LAT, "started");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();
  auto spawn_time = sp_clnt->ProcEnv()->GetSpawnTime();
  log(SPAWN_LAT, "Time since spawn until main: {}s", t.seconds() - spawn_time.seconds() + (t.nanos() - spawn_time.nanos()) / (1E9));

  // Create the echo server
  auto srv = std::make_shared<sigmaos::apps::spin::Srv>(sp_clnt);
  // Run the server
  srv->Run();
}
