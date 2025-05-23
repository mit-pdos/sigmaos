#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>
#include <optional>
#include <map>
#include <future>
#include <mutex>

#include <serr/serr.h>
#include <io/transport/call.h>

namespace sigmaos {
namespace io::demux::internal {

class CallPromise {
  public:
  CallPromise(std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>> p) : _p(std::move(p)) {}
  ~CallPromise() {}

  std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>> Get() {
    auto p = std::move(_p);
    _p = nullptr;
    return std::move(p);
  }
  private:
  std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>> _p;
};

class CallMap {
  public:
  CallMap() : _mu(), _closed(false), _calls() {}
  ~CallMap() {}

  std::expected<int, sigmaos::serr::Error> Put(uint64_t seqno, std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>> future);
  std::optional<std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>>> Remove(uint64_t seqno);
  std::vector<uint64_t> Outstanding();
  void Close();
  bool IsClosed();

  private:
  std::mutex _mu;
  bool _closed;
  std::map<int, std::unique_ptr<CallPromise>> _calls;
};

};
};
