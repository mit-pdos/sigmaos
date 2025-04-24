#pragma once

#include <vector>
#include <expected>

#include <io/conn/conn.h>

namespace sigmaos {
namespace io::frame {

// Read frames
std::expected<int, std::string> ReadFrameIntoVec(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<unsigned char> b);
std::expected<int, std::string> ReadFramesIntoIOVec(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::vector<unsigned char>> iov);
std::expected<uint64_t, std::string> ReadSeqno(std::unique_ptr<sigmaos::io::conn::UnixConn> conn);
std::expected<uint32_t, std::string> ReadNumFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn);

// Write frames
std::expected<int, std::string> WriteFrame(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<unsigned char> b);
std::expected<int, std::string> WriteFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, std::vector<std::vector<unsigned char>> iov);
std::expected<int, std::string> WriteSeqno(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, uint64_t seqno);
std::expected<int, std::string> WriteNumFrames(std::unique_ptr<sigmaos::io::conn::UnixConn> conn, uint32_t nframes);

};
};
