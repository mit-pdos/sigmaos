#include <io/transport/transport.h>

#include <io/frame/frame.h>

namespace sigmaos {
namespace io::transport {

std::expected<int, std::string> Transport::WriteCall(const Call &c) {
  auto res = sigmaos::io::frame::WriteSeqno(_conn, c.GetSeqno());
  if (!res.has_value()) {
    return res;
  }
  res = sigmaos::io::frame::WriteFrames(_conn, c.GetIOVec());
  if (!res.has_value()) {
    return res;
  }
  return 0;
}

std::expected<std::shared_ptr<Call>, std::string> Transport::ReadCall(std::vector<std::vector<unsigned char>> &iov) {
  uint64_t seqno;
  uint32_t nframes;
  {
    auto res = sigmaos::io::frame::ReadSeqno(_conn);
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
    seqno = res.value();
  }
  {
    auto res = sigmaos::io::frame::ReadNumFrames(_conn);
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
    nframes = res.value();
  }
  // Resize the iov according to the incoming number of frames
  iov.resize(nframes);
  auto res = sigmaos::io::frame::ReadFramesIntoIOVec(_conn, iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return std::make_shared<Call>(seqno, iov);
}

};
};
