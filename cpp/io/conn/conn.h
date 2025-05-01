#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <vector>
#include <expected>

#include <util/log/log.h>

namespace sigmaos {
namespace io::conn {

const std::string CONN = "CONN";
const std::string CONN_ERR = "CONN" + sigmaos::util::log::ERR;

class UnixConn {
  public:
  // Create a unix socket connection
  UnixConn(std::string pn) {
    log(CONN, "New unix connection {}", pn);
    _sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (_sockfd == -1) {
      throw std::runtime_error("Failed to create spproxy socket fd");
    }
    _addr.sun_family = AF_UNIX;
    strncpy(_addr.sun_path, pn.c_str(), sizeof(_addr.sun_path) - 1);
    _addr.sun_path[sizeof(_addr.sun_path) - 1] = '\0';
    if (connect(_sockfd, (struct sockaddr *) &_addr, sizeof(_addr)) == -1) {
      close(_sockfd);
      throw std::runtime_error("Failed to connect to spproxy socket");
    }
  }

  // Read/Write a buffer
  std::expected<int, std::string> Read(std::vector<unsigned char> &b);
  std::expected<int, std::string> Write(const std::vector<unsigned char> &b);

  // Read/Write a number
  std::expected<uint32_t, std::string> ReadUint32();
  std::expected<int, std::string> WriteUint32(uint32_t i);
  std::expected<uint64_t, std::string> ReadUint64();
  std::expected<int, std::string> WriteUint64(uint64_t i);

  // Close a connection
  std::expected<int, std::string> Close();

  ~UnixConn() { Close(); }

  private:
  int _sockfd;
  sockaddr_un _addr;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<int, std::string> read_bytes(unsigned char *b, size_t size);
  std::expected<int, std::string> write_bytes(const unsigned char *b, size_t size);
};

};
};
