#include <iostream>

#include <proxy/sigmap/sigmap.h>

int main() {
  std::cout << "Running" << std::endl;
  auto sp_clnt = std::unique_ptr<sigmaos::proxy::sigmap::Clnt>(new sigmaos::proxy::sigmap::Clnt());
  return 1;
  return 0;
}
