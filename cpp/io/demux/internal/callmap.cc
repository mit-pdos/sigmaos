#include <io/demux/internal/callmap.h>

namespace sigmaos {
namespace io::demux::internal {

std::expected<int, sigmaos::serr::Error> CallMap::Put(uint64_t seqno, std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>> p) {
  std::lock_guard<std::mutex> guard(_mu);
  if (_closed) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, "Err: demux closed"));
  }
  _calls[seqno] = std::move(p);
  return 0;
}

std::optional<std::unique_ptr<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>>> CallMap::Remove(uint64_t seqno) {
  if (!_calls.contains(seqno)) {
    return std::nullopt;
  }
  auto call = std::move(_calls.extract(seqno).mapped());
  return call;
}

std::vector<uint64_t> CallMap::Outstanding() {
  std::vector<uint64_t> outstanding(_calls.size());
  int i = 0;
  for (const auto &pair: _calls) {
    outstanding[i] = pair.first;
  }
  return outstanding;
}

void CallMap::Close() {
  std::lock_guard<std::mutex> guard(_mu);
  _closed = true;
}

bool CallMap::IsClosed() {
  std::lock_guard<std::mutex> guard(_mu);
  return _closed;
}

};
};
