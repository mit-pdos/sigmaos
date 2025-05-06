#pragma once

#include <vector>
#include <expected>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/conn/conn.h>

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
std::expected<int, sigmaos::serr::Error> ReadFrameIntoVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::string *b);
std::expected<int, sigmaos::serr::Error> ReadFramesIntoIOVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::string *> &iov);
std::expected<uint64_t, sigmaos::serr::Error> ReadSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn);
std::expected<uint32_t, sigmaos::serr::Error> ReadNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn);

// Write frames
std::expected<int, sigmaos::serr::Error> WriteFrame(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::string *b);
std::expected<int, sigmaos::serr::Error> WriteFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::vector<std::string *> &iov);
std::expected<int, sigmaos::serr::Error> WriteSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint64_t seqno);
std::expected<int, sigmaos::serr::Error> WriteNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint32_t nframes);

};
};
