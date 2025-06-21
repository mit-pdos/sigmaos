#pragma once

#include <mutex>
#include <memory>
#include <format>
#include <string>

#include <iostream>

#include <spdlog/sinks/base_sink.h>
#include <spdlog/sinks/stdout_sinks.h>
#include <spdlog/spdlog.h>
#include <spdlog/common.h>

#include <util/common/util.h>

// Some common debug selectors
const std::string TEST = "TEST";
const std::string ALWAYS = "ALWAYS";
const std::string FATAL = "FATAL";
const std::string SPAWN_LAT = "SPAWN_LAT";
const std::string PROXY_RPC_LAT = "PROXY_RPC_LAT";

namespace sigmaos {
namespace util::log {

// Initialize a logger with a debug selector
bool init_logger(std::string selector);

const std::string ERR = "_ERR";

class sigmadebug_sink : public spdlog::sinks::base_sink<std::mutex> {
  public:
  sigmadebug_sink(std::string selector) : _enabled(false), _stdout_sink(std::make_shared<spdlog::sinks::stdout_sink_mt>()) {
    std::string sigmadebug(std::getenv("SIGMADEBUG"));
    std::string pid(std::getenv("SIGMADEBUGPID"));
    _stdout_sink->set_pattern(std::format("%H:%M:%S.%f {} {} %v", pid, selector));
    if (selector == ALWAYS || selector == FATAL) {
      _enabled = true;
    } else {
      _enabled = sigmaos::util::common::ContainsLabel(sigmadebug, selector);
    }
  }
  void sink_it_(const spdlog::details::log_msg& msg) override {
    if (_enabled) {
      _stdout_sink->log(msg);
    }
  }
  void flush_() override { _stdout_sink->flush(); }

  protected:
  private:
  bool _enabled;
  std::shared_ptr<spdlog::sinks::stdout_sink_mt> _stdout_sink;
};

// Used to initialize some common debug selectors
class _log {
  public:
  _log();
  ~_log();
  private:
  static bool _l_always;
  static bool _l_fatal;
  static bool _l_test;
  static bool _l_spawn_lat;
};

};
};

// Write a log line given a selector
template <typename... Args>
void log(std::string selector, spdlog::format_string_t<Args...> fmt, Args &&...args) {
  auto logger = spdlog::get(selector);
  if (logger == nullptr) {
    sigmaos::util::log::init_logger(selector);
    logger = spdlog::get(selector);
  }
  logger->info(fmt, std::forward<Args>(args)...);
}

// Write a log line given a selector
template <typename... Args>
[[noreturn]] void fatal(spdlog::format_string_t<Args...> fmt, Args &&...args) {
  auto logger = spdlog::get(FATAL);
  if (logger == nullptr) {
    sigmaos::util::log::init_logger(FATAL);
    logger = spdlog::get(FATAL);
  }
  logger->info(fmt, std::forward<Args>(args)...);
  throw std::runtime_error("FATAL CPP");
}
