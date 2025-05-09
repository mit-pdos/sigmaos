#pragma once

#include <expected>

#include <serr/serr.h>

namespace sigmaos {
namespace io::conn {

class Conn {
public:
  // Read/Write a buffer
  virtual std::expected<int, sigmaos::serr::Error> Read(std::string *b) = 0;
  virtual std::expected<int, sigmaos::serr::Error> Write(const std::string *b) = 0;

  // Read/Write a number
  virtual std::expected<uint32_t, sigmaos::serr::Error> ReadUint32() = 0;
  virtual std::expected<int, sigmaos::serr::Error> WriteUint32(uint32_t i) = 0;
  virtual std::expected<uint64_t, sigmaos::serr::Error> ReadUint64() = 0;
  virtual std::expected<int, sigmaos::serr::Error> WriteUint64(uint64_t i) = 0;

  // Close a connection
  virtual std::expected<int, sigmaos::serr::Error> Close() = 0;

private:
};

};
};
