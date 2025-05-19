#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <rpc/srv.h>
#include <sigmap/const.h>

#include <apps/spin/srv.h>

int main(int argc, char *argv[]) {
  sigmaos::util::log::init_logger(sigmaos::apps::spin::SPINSRV);
  log(sigmaos::apps::spin::SPINSRV, "main");
  log(SPAWN_LAT, "started");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  // Create the echo server
  auto srv = std::make_shared<sigmaos::apps::spin::Srv>(sp_clnt);
  // Run the server
  srv->Run();
}
