#include <io/frame/frame.h>

namespace sigmaos {
namespace io::frame {

bool Frame::_l = sigmaos::util::log::init_logger(FRAME);
bool Frame::_l_e = sigmaos::util::log::init_logger(FRAME_ERR);

// Read frames
std::expected<int, sigmaos::serr::Error> ReadFrameIntoBuf(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    std::shared_ptr<sigmaos::io::iovec::Buffer> buf) {
  std::string *b = buf->Get();
  uint32_t nbyte = 0;
  auto res = conn->ReadUint32();
  if (!res.has_value()) {
    return res;
  }
  nbyte = res.value();
  if (nbyte < 4) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, "nbyte < 4"));
  }
  nbyte -= 4;
  log(FRAME, "ReadFrameIntoBuf nbyte {} iov 0x{:x}", nbyte,
      (uint64_t)b->data());
  // If the vector passed in had no underlying buffer, resize it
  if (b->size() == 0) {
    b->resize(nbyte);
  }
  if (nbyte > b->size()) {
    fatal("Try to read {} bytes into a {}-byte buffer", nbyte, b->size());
  }
  // Shrink the buffer to the right size
  b->resize(nbyte);
  res = conn->Read(b);
  if (!res.has_value()) {
    return res;
  }
  if (res.value() != nbyte) {
    return std::unexpected(
        sigmaos::serr::Error(sigmaos::serr::TErrUnreachable,
                             std::format("Read wrong number of bytes: {} != {}",
                                         nbyte, res.value())));
  }
  return nbyte;
}

std::expected<int, sigmaos::serr::Error> ReadFramesIntoIOVec(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    std::shared_ptr<sigmaos::io::iovec::IOVec> iov) {
  for (int i = 0; i < iov->Size(); i++) {
    auto res = ReadFrameIntoBuf(conn, iov->GetBuffer(i));
    if (!res.has_value()) {
      return res;
    }
  }
  return 0;
}

std::expected<uint64_t, sigmaos::serr::Error> ReadSeqno(
    std::shared_ptr<sigmaos::io::conn::Conn> conn) {
  auto res = conn->ReadUint64();
  if (!res.has_value()) {
    return res;
  }
  uint64_t seqno = res.value();
  // Suspiciously large seqno, so print more info
  if (seqno > 10000000) {
    log(FRAME, "ReadSeqno [{}] {}", conn->GetID(), seqno);
  } else {
    log(FRAME, "ReadSeqno {}", seqno);
  }
  return seqno;
}

std::expected<uint32_t, sigmaos::serr::Error> ReadNumFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn) {
  auto res = conn->ReadUint32();
  if (!res.has_value()) {
    return res;
  }
  uint32_t nframes = res.value();
  log(FRAME, "ReadNumFrames {}", nframes);
  if (nframes > 10000000) {
    log(ALWAYS, "ReadNumFrames [{}] {}", conn->GetID(), (int)nframes);
  }
  return nframes;
}

// Write frames
std::expected<int, sigmaos::serr::Error> WriteFrame(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    const std::shared_ptr<sigmaos::io::iovec::Buffer> buf) {
  auto b = buf->Get();
  uint32_t nbyte = b->size() + 4;

  log(FRAME, "WriteFrame sz {}", nbyte);
  auto res = conn->WriteUint32(nbyte);
  if (!res.has_value()) {
    return res;
  }
  return conn->Write(b);
}

std::expected<int, sigmaos::serr::Error> WriteFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    std::shared_ptr<sigmaos::io::iovec::IOVec> iov) {
  log(FRAME, "WriteFrames numFrames {}", iov->Size());
  // Write the number of frames
  auto res = conn->WriteUint32(iov->Size());
  if (!res.has_value()) {
    return res;
  }
  for (int i = 0; i < iov->Size(); i++) {
    auto buf = iov->GetBuffer(i);
    log(FRAME, "WriteFrames next frame len {}", buf->Size());
    auto res = WriteFrame(conn, buf);
    if (!res.has_value()) {
      return res;
    }
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> WriteSeqno(
    std::shared_ptr<sigmaos::io::conn::Conn> conn, uint64_t seqno) {
  auto res = conn->WriteUint64(seqno);
  if (!res.has_value()) {
    return res;
  }
  return res.value();
}

std::expected<int, sigmaos::serr::Error> WriteNumFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn, uint32_t nframes) {
  auto res = conn->WriteUint32(nframes);
  if (!res.has_value()) {
    return res;
  }
  return res.value();
}

};  // namespace io::frame
};  // namespace sigmaos
