#include <io/frame/frame.h>
#include <io/transport/transport.h>

namespace sigmaos {
namespace io::transport {

bool Transport::_l = sigmaos::util::log::init_logger(TRANSPORT);
bool Transport::_l_e = sigmaos::util::log::init_logger(TRANSPORT_ERR);

std::expected<int, sigmaos::serr::Error> Transport::WriteCall(
    std::shared_ptr<Call> call) {
  auto res = _calls.Put(call->GetSeqno(), call);
  if (!res.has_value()) {
    return res;
  }
  log(TRANSPORT, "WriteCall seqno {}", (int)call->GetSeqno());
  res = sigmaos::io::frame::WriteSeqno(_conn, call->GetSeqno());
  if (!res.has_value()) {
    return res;
  }
  log(TRANSPORT, "WriteCall iniov len {}", call->GetInIOVec()->Size());
  res = sigmaos::io::frame::WriteFrames(_conn, call->GetInIOVec());
  if (!res.has_value()) {
    return res;
  }
  return 0;
}

std::expected<std::shared_ptr<Call>, sigmaos::serr::Error>
Transport::ReadCall() {
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
  std::shared_ptr<Call> call;
  {
    auto res = _calls.Remove(seqno);
    // If no existing call was present, then create one (this must be a reader
    // thread for demuxsrv)
    if (!res.has_value()) {
      auto in_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
      auto out_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
      // Make room in the iovec to read the buffers
      out_iov->AddBuffers(nframes);
      call = std::make_shared<Call>(seqno, in_iov, out_iov);
    } else {
      call = res.value();
    }
  }
  auto out_iov = call->GetOutIOVec();
  if (out_iov->Size() < nframes) {
    log(TRANSPORT_ERR,
        "Size of supplied out_iov ({}) less than number of frames to be read ({})",
        out_iov->Size(), nframes);
    fatal("Size of supplied out_iov ({}) less than number of frames to be read ({})",
          out_iov->Size(), nframes);
  }
  auto res = sigmaos::io::frame::ReadFramesIntoIOVec(_conn, nframes, out_iov);
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

};  // namespace io::transport
};  // namespace sigmaos
