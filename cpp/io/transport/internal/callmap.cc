#include <io/transport/internal/callmap.h>

namespace sigmaos {
namespace io::transport::internal {

std::expected<int, sigmaos::serr::Error> CallMap::Put(
    uint64_t seqno, std::shared_ptr<sigmaos::io::transport::Call> call) {
  std::lock_guard<std::mutex> guard(_mu);
  if (_closed) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable,
                                                "Err: iovecmap closed"));
  }
  _calls[seqno] = call;
  return 0;
}

std::optional<std::shared_ptr<sigmaos::io::transport::Call>> CallMap::Remove(
    uint64_t seqno) {
  if (!_calls.contains(seqno)) {
    return std::nullopt;
  }
  return _calls.extract(seqno).mapped();
}

std::expected<int, sigmaos::serr::Error> CallMap::Close() {
  std::lock_guard<std::mutex> guard(_mu);
  _closed = true;
  return 0;
}

bool CallMap::IsClosed() {
  std::lock_guard<std::mutex> guard(_mu);
  return _closed;
}

}  // namespace io::transport::internal
};  // namespace sigmaos
