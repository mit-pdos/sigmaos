#pragma once

#include <io/conn/conn.h>
#include <io/iovec/iovec.h>
#include <serr/serr.h>
#include <util/log/log.h>

#include <expected>
#include <vector>

namespace sigmaos {
namespace io::frame {

const std::string FRAME = "FRAME";
const std::string FRAME_ERR = "FRAME" + sigmaos::util::log::ERR;

class Frame {
 public:
  Frame() {};
  ~Frame() {};

 private:
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

// Read frames
std::expected<int, sigmaos::serr::Error> ReadFrameIntoBuf(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    std::shared_ptr<sigmaos::io::iovec::Buffer> buf);
std::expected<int, sigmaos::serr::Error> ReadFramesIntoIOVec(
    std::shared_ptr<sigmaos::io::conn::Conn> conn, uint32_t nframes,
    std::shared_ptr<sigmaos::io::iovec::IOVec> iov);
std::expected<uint64_t, sigmaos::serr::Error> ReadSeqno(
    std::shared_ptr<sigmaos::io::conn::Conn> conn);
std::expected<uint32_t, sigmaos::serr::Error> ReadNumFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn);

// Write frames
std::expected<int, sigmaos::serr::Error> WriteFrame(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    const std::shared_ptr<sigmaos::io::iovec::Buffer> buf);
std::expected<int, sigmaos::serr::Error> WriteFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn,
    const std::shared_ptr<sigmaos::io::iovec::IOVec> iov);
std::expected<int, sigmaos::serr::Error> WriteSeqno(
    std::shared_ptr<sigmaos::io::conn::Conn> conn, uint64_t seqno);
std::expected<int, sigmaos::serr::Error> WriteNumFrames(
    std::shared_ptr<sigmaos::io::conn::Conn> conn, uint32_t nframes);

};  // namespace io::frame
};  // namespace sigmaos
