#include <util/log/log.h>

namespace sigmaos {
namespace util::log {

std::mutex _mu;

bool init_logger(std::string selector) {
  std::lock_guard<std::mutex> guard(_mu);
  auto log = spdlog::get(selector);
  // If this logger hasn't already been initialized, create a new one and
  // register it.
  if (!log) {
    auto sdbg_sink = std::make_shared<sigmaos::util::log::sigmadebug_sink>(selector);
    log = std::make_shared<spdlog::logger>(selector, sdbg_sink);
    spdlog::register_logger(log);
    return true;
  }
  return false;
}

};
};
