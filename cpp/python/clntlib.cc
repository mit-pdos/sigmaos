#include <cstdlib>

#include <python/clntlib.h>

#include <proc/status.h>
#include <proxy/sigmap/sigmap.h>

std::unique_ptr<sigmaos::proxy::sigmap::Clnt> clnt;

void init_socket() {
  clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();
}

void started() {
  clnt->Started();
}

void exited()
{
  sigmaos::proc::Tstatus status = sigmaos::proc::Tstatus::StatusOK;
  std::string msg = "Exited normally!";
  clnt->Exited(status, msg);
}

void stat_stub(char* path) {
  clnt->Test();
}
