#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>
#include <serr/serr.h>

const std::string CPP_USER_PROC = "CPP_USER_PROC";

int main() {
  sigmaos::util::log::init_logger(CPP_USER_PROC);
  log(CPP_USER_PROC, "Running");
  auto sp_clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();

  {
    auto res = sp_clnt->Started();
    if (!res.has_value()) {
      log(CPP_USER_PROC, "Error started: {}", res.error());
    }
  }

  // Test connection to spproxyd
  log(CPP_USER_PROC, "Test");
  sp_clnt->Test();
  log(CPP_USER_PROC, "Done testing");
  // TODO: exit with status
  {
    auto res = sp_clnt->Exited();
    if (!res.has_value()) {
      log(CPP_USER_PROC, "Error started: {}", res.error());
    }
  }
  std::exit(0);
  return 0;
}
