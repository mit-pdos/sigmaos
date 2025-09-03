#pragma once

#include <serr/serr.h>
#include <util/log/log.h>

#include <expected>

namespace sigmaos {
namespace io::conn {

class Conn {
 public:
  Conn(std::string id) : Conn(id, -1) {}
  Conn(std::string id, int sockfd) : _id(id), _sockfd(sockfd) {}
  ~Conn() {}

  // Get conn ID
  std::string GetID() { return _id; }

  // Read/Write a buffer
  std::expected<int, sigmaos::serr::Error> Read(std::string *b);
  std::expected<int, sigmaos::serr::Error> Write(const std::string *b);

  // Read/Write a number
  std::expected<uint32_t, sigmaos::serr::Error> ReadUint32();
  std::expected<int, sigmaos::serr::Error> WriteUint32(uint32_t i);
  std::expected<uint64_t, sigmaos::serr::Error> ReadUint64();
  std::expected<int, sigmaos::serr::Error> WriteUint64(uint64_t i);

  // Close a connection
  std::expected<int, sigmaos::serr::Error> Close();

 protected:
  void init(int sockfd) { _sockfd = sockfd; }
  std::expected<int, sigmaos::serr::Error> read_bytes(char *b, size_t size);
  std::expected<int, sigmaos::serr::Error> write_bytes(const char *b,
                                                       size_t size);

 private:
  std::string _id;
  int _sockfd;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};  // namespace io::conn
};  // namespace sigmaos
