#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <rpc/srv.h>
#include <sigmap/const.h>

#include <apps/echo/srv.h>

int main(int argc, char *argv[]) {
  sigmaos::util::log::init_logger(sigmaos::apps::echo::ECHOSRV);
  log(sigmaos::apps::echo::ECHOSRV, "main");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  // Create the echo server
  auto srv = std::make_shared<sigmaos::apps::echo::Srv>(sp_clnt);
  // Run the server
  srv->Run();
}
