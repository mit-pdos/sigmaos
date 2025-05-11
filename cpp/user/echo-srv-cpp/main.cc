#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>
#include <proc/proc.h>
#include <io/conn/tcp/tcp.h>

const std::string ECHOSRV = "ECHOSRV";

void wait_for_eviction(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) {
  log(ECHOSRV, "Waiting for eviction");
  auto res = sp_clnt->WaitEvict();
  if (!res.has_value()) {
    log(ECHOSRV, "Error WaitEvict: {}", res.error());
  }
  log(ECHOSRV, "Done waiting for eviction");
}

int main(int argc, char *argv[]) {
  sigmaos::util::log::init_logger(ECHOSRV);
  log(ECHOSRV, "Running");
  auto sp_clnt = std::make_shared<sigmaos::proxy::sigmap::Clnt>();

  std::thread evict_thread(wait_for_eviction, sp_clnt);

  log(ECHOSRV, "Starting TCP server");
  auto l = sigmaos::io::conn::tcpconn::Listener();
  log(ECHOSRV, "TCP server started");
  {
    auto res = sp_clnt->Started();
    if (!res.has_value()) {
      log(ECHOSRV, "Error started: {}", res.error());
    }
  }
  log(ECHOSRV, "Server started");

  std::string msg("Evicted! Done serving.");
  sigmaos::proc::Tstatus exit_status = sigmaos::proc::Tstatus::StatusEvicted;

  {
    auto res = sp_clnt->Exited(exit_status, msg);
    if (!res.has_value()) {
      log(ECHOSRV, "Error exited: {}", res.error());
    }
  }
  std::exit(0);
}
