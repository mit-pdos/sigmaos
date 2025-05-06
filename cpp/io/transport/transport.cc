#include <io/transport/transport.h>

#include <io/frame/frame.h>

namespace sigmaos {
namespace io::transport {

bool Transport::_l = sigmaos::util::log::init_logger(TRANSPORT);
bool Transport::_l_e = sigmaos::util::log::init_logger(TRANSPORT_ERR);

std::expected<int, sigmaos::serr::Error> Transport::WriteCall(std::shared_ptr<Call> call) {
  auto res = _calls.Put(call->GetSeqno(), call);
  if (!res.has_value()) {
    return res;
  }
  log(TRANSPORT, "WriteCall seqno {}", call->GetSeqno());
  res = sigmaos::io::frame::WriteSeqno(_conn, call->GetSeqno());
  if (!res.has_value()) {
    return res;
  }
  log(TRANSPORT, "WriteCall iniov len {}", call->GetInIOVec().size());
  res = sigmaos::io::frame::WriteFrames(_conn, call->GetInIOVec());
  if (!res.has_value()) {
    return res;
  }
  return 0;
}

std::expected<std::shared_ptr<Call>, sigmaos::serr::Error> Transport::ReadCall() {
  uint64_t seqno;
  uint32_t nframes;
  {
    auto res = sigmaos::io::frame::ReadSeqno(_conn);
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
    seqno = res.value();
    log(TRANSPORT, "ReadCall seqno {}", seqno);
  }
  {
    auto res = sigmaos::io::frame::ReadNumFrames(_conn);
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
    nframes = res.value();
    log(TRANSPORT, "ReadCall seqno {} nframes {}", seqno, nframes);
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


std::expected<int, sigmaos::serr::Error> Transport::Close() {
//  _calls.Close(); // XXX never called in the go implementation
  log(TRANSPORT, "Close");
  auto res = _conn->Close();
  log(TRANSPORT, "Done close");
  return res;
}

};
};
