#include <io/transport/callmap.h>

namespace sigmaos {
namespace io::transport::internal {

std::expected<int, std::string> CallMap::Put(uint64_t seqno, std::shared_ptr<sigmaos::io::transport::Call> call) {
  std::lock_guard<std::mutex> guard(_mu);
  if (_closed) {
    return std::unexpected("Err: iovecmap closed");
  }
  _calls[seqno] = call;
  return 0;
}

std::optional<std::shared_ptr<sigmaos::io::transport::Call>> CallMap::Remove(uint64_t seqno) {
  if (!_calls.contains(seqno)) {
    return std::nullopt;
  }
  return _calls.extract(seqno).mapped();
}

std::expected<int, std::string> CallMap::Close() {
  std::lock_guard<std::mutex> guard(_mu);
  _closed = true;
}

bool CallMap::IsClosed() {
  std::lock_guard<std::mutex> guard(_mu);
  return _closed;
}

}
};
