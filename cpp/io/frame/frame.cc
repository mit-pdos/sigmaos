#include <io/frame/frame.h>



// Read frames
std::expected<int, std::string> ReadFrameIntoVec(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, std::string> ReadFramesIntoIOVec(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::vector<unsigned char>> iov) {
  throw std::runtime_error("unimplemented");
}


std::expected<uint64_t, std::string> ReadSeqno(std::unique_ptr<sigmaos::io::conn::UnixConn> conn) {
  auto res = conn->ReadUint64();
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return res.value();
}

std::expected<uint32_t, std::string> ReadNumFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn) {
  throw std::runtime_error("unimplemented");
}

// Write frames
std::expected<int, std::string> WriteFrame(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<unsigned char> b) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, std::string> WriteFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::vector<unsigned char>> iov) {
  throw std::runtime_error("unimplemented");
}

std::expected<int, std::string> WriteSeqno(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, uint64_t seqno) {
  auto res = conn->WriteUint64(seqno);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return res.value();
}

std::expected<int, std::string> WriteNumFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, uint32_t nframes) {
  throw std::runtime_error("unimplemented");
}
