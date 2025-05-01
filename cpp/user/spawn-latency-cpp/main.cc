#include <iostream>

#include <proxy/sigmap/sigmap.h>
#include <util/log/log.h>

int main() {
  auto sdbg_sink = std::make_shared<sigmaos::util::log::sigmadebug_sink>("TEST234");
  auto log = std::make_shared<spdlog::logger>("main", sdbg_sink);
  spdlog::register_logger(log);
  spdlog::get("main")->info("test spdlog log");
  std::cout << "Running" << std::endl;
  auto sp_clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();

  // Test connection to spproxyd
  sp_clnt->Test();
  std::cout << "done testing" << std::endl;
  return 1;
  return 0;
}
