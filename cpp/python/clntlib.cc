#include <cstdlib>

#include <python/clntlib.h>

#include <proxy/sigmap/sigmap.h>

const char* api_socket_env_var = "SIGMA_PYAPI_FD";
int api_sfd = 0; 

std::unique_ptr<sigmaos::proxy::sigmap::Clnt> clnt;

void init_socket() {
  std::cout << "NEW CPYLIB" << std::endl;
  
  // For started() and exited()
  const char* sfd_str = std::getenv(api_socket_env_var);
  if (sfd_str == NULL) {
    exit(-1);
  }
  api_sfd = std::atoi(sfd_str);

  // SPProxyClnt
  clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();
  // clnt->init_conn();
}

void started() {
  char response[2];
  write(api_sfd, "api/started\n", 12);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

void exited()
{
  char response[2];
  write(api_sfd, "api/exited\n", 11);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

void stat_stub(char* path) {
  clnt->Test();
}
