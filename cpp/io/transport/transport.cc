#include <io/transport/transport.h>

#include <io/frame/frame.h>

namespace sigmaos {
namespace io::transport {

std::expected<int, std::string> Transport::WriteCall(std::shared_ptr<Call> call) {
  auto res = _calls.Put(call->GetSeqno(), call);
  if (!res.has_value()) {
    return res;
  }
  std::cout << "Transport::WriteCall seqno " << call->GetSeqno() << std::endl;
  res = sigmaos::io::frame::WriteSeqno(_conn, call->GetSeqno());
  if (!res.has_value()) {
    return res;
  }
  std::cout << "Transport::WriteCall iovec len " << call->GetInIOVec().size() << std::endl;
  res = sigmaos::io::frame::WriteFrames(_conn, call->GetInIOVec());
  if (!res.has_value()) {
    return res;
  }
  return 0;
}

std::expected<std::shared_ptr<Call>, std::string> Transport::ReadCall() {
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
  auto call = _calls.Remove(seqno).value();
  // Resize the iov according to the incoming number of frames
  call->GetOutIOVec().resize(nframes);
  auto res = sigmaos::io::frame::ReadFramesIntoIOVec(_conn, call->GetOutIOVec());
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return call;
}


std::expected<int, std::string> Transport::Close() {
//  _calls.Close(); // XXX never called in the go implementation
  std::cout << "Closing transport" << std::endl;
  auto res = _conn->Close();
  std::cout << "Done closing transport" << std::endl;
  return res;
}

};
};
