#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>
#include <map>
#include <future>
#include <mutex>

#include <io/transport/call.h>

namespace sigmaos {
namespace io::demux::internal {

class CallMap {
  public:
  CallMap() : _mu(), _closed(false), _calls() {}
  ~CallMap() {}

  std::expected<int, std::string> Put(uint64_t seqno, std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string>>> future);
  std::optional<std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string>>>> Remove(uint64_t seqno);
  std::vector<uint64_t> Outstanding();
  std::expected<int, std::string> Close();
  bool IsClosed();

  private:
  std::mutex _mu;
  bool _closed;
  std::map<uint64_t, std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string>>>> _calls;
};

};
};
