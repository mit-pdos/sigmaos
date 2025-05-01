#pragma once

#include <vector>
#include <expected>

#include <io/conn/conn.h>
#include <util/log/log.h>

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
std::expected<int, std::string> ReadFrameIntoVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<unsigned char> &b);
std::expected<int, std::string> ReadFramesIntoIOVec(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::vector<unsigned char>> &iov);
std::expected<uint64_t, std::string> ReadSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn);
std::expected<uint32_t, std::string> ReadNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn);

// Write frames
std::expected<int, std::string> WriteFrame(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::vector<unsigned char> &b);
std::expected<int, std::string> WriteFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, const std::vector<std::vector<unsigned char>> &iov);
std::expected<int, std::string> WriteSeqno(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint64_t seqno);
std::expected<int, std::string> WriteNumFrames(std::shared_ptr<sigmaos::io::conn::UnixConn> conn, uint32_t nframes);

};
};
