#include <iostream>

#include <util/log/log.h>
#include <proxy/sigmap/sigmap.h>

const std::string CPP_USER_PROC = "CPP_USER_PROC";

int main() {
  sigmaos::util::log::init_logger(CPP_USER_PROC);
  log(CPP_USER_PROC, "Running");
  auto sp_clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();

  // Test connection to spproxyd
  log(CPP_USER_PROC, "Test");
  sp_clnt->Test();
  log(CPP_USER_PROC, "Done testing");
  return 1;
  return 0;
}
