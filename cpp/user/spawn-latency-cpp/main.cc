#include <iostream>

#include <proxy/sigmap/sigmap.h>

int main() {
  std::cout << "Running" << std::endl;
  auto sp_clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();

  // Test connection to spproxyd
  sp_clnt->Test();
  std::cout << "done testing" << std::endl;
  return 1;
  return 0;
}
