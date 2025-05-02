#pragma once

#include <memory>
#include <map>
#include <mutex>
#include <expected>
#include <optional>

#include <serr/serr.h>
#include <io/transport/call.h>

namespace sigmaos {
namespace io::transport::internal {

class CallMap {
  public:
  CallMap() : _mu(), _closed(false), _calls() {}
  ~CallMap() {}

  std::expected<int, sigmaos::serr::Error> Put(uint64_t seqno, std::shared_ptr<sigmaos::io::transport::Call> call);
  std::optional<std::shared_ptr<sigmaos::io::transport::Call>> Remove(uint64_t seqno);
  std::expected<int, sigmaos::serr::Error> Close();
  bool IsClosed();

  private:
  std::mutex _mu;
  bool _closed;
  std::map<uint64_t, std::shared_ptr<sigmaos::io::transport::Call>> _calls;
};

};
};


