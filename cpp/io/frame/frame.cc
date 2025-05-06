#include <io/frame/frame.h>

namespace sigmaos {
namespace io::frame {

bool Frame::_l = sigmaos::util::log::init_logger(FRAME);
bool Frame::_l_e = sigmaos::util::log::init_logger(FRAME_ERR);

// Read frames
std::expected<int, sigmaos::serr::Error> ReadFrameIntoVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::string *b) {
  uint32_t nbyte = 0;

  auto res = conn->ReadUint32();
  if (!res.has_value()) {
    return res;
  }
  nbyte = res.value();
  if (nbyte < 4) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, "nbyte < 4"));
  }
  nbyte -= 4;
  log(FRAME, "ReadFrameIntoVec nbyte {} iov 0x{:x}", nbyte, (uint64_t) b);
  // If the vector passed in had no underlying buffer, resize it
  if (b->size() == 0) {
    b->resize(nbyte);
  }
  if (nbyte > b->size()) {
    throw std::runtime_error(std::format("Try to read {} bytes into a {}-byte buffer", nbyte, b->size()));
  }
  // Shrink the buffer to the right size
  b->resize(nbyte);
  res = conn->Read(b);
  if (!res.has_value()) {
    return res;
  }
  if (res.value() != nbyte) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, std::format("Read wrong number of bytes: {} != {}", nbyte, res.value())));
  }
  return nbyte;
}

std::expected<int, sigmaos::serr::Error> ReadFramesIntoIOVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::string *> &iov) {
  for (auto &b : iov) {
    auto res = ReadFrameIntoVec(conn, b);
    if (!res.has_value()) {
      return res;
    }
  }
  return 0;
}


std::expected<uint64_t, sigmaos::serr::Error> ReadSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn) {
  auto res = conn->ReadUint64();
  if (!res.has_value()) {
    return res;
  }
  uint64_t seqno = res.value();
  log(FRAME, "ReadSeqno nbyte {}", seqno);
  return seqno;
}

std::expected<uint32_t, sigmaos::serr::Error> ReadNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn) {
  auto res = conn->ReadUint32();
  if (!res.has_value()) {
    return res;
  }
  uint32_t nframes = res.value();
  log(FRAME, "ReadNumFrames nbyte {}", nframes);
  return nframes;
}

// Write frames
std::expected<int, sigmaos::serr::Error> WriteFrame(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::string *b) {
  uint32_t nbyte = b->size() + 4;

  log(FRAME, "WriteFrame sz {}", nbyte);
  auto res = conn->WriteUint32(nbyte);
  if (!res.has_value()) {
    return res;
  }
  return conn->Write(b);
}

std::expected<int, sigmaos::serr::Error> WriteFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::vector<std::string *> &iov) {
  log(FRAME, "WriteFrames numFrames {}", iov.size());
  // Write the number of frames
  auto res = conn->WriteUint32(iov.size());
  if (!res.has_value()) {
    return res;
  }
  for (const auto &b : iov) {
    log(FRAME, "WriteFrames next frame len {}", b->size());
    auto res = WriteFrame(conn, b);
    if (!res.has_value()) {
      return res;
    }
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> WriteSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint64_t seqno) {
  auto res = conn->WriteUint64(seqno);
  if (!res.has_value()) {
    return res;
  }
  return res.value();
}

std::expected<int, sigmaos::serr::Error> WriteNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint32_t nframes) {
  auto res = conn->WriteUint32(nframes);
  if (!res.has_value()) {
    return res;
  }
  return res.value();
}

};
};
