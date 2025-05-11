#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>
#include <netinet/in.h>

#include <iostream>
#include <memory>
#include <vector>
#include <expected>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <serr/serr.h>

namespace sigmaos {
namespace io::conn::tcpconn {

const int SOCK_BACKLOG = 5;

const std::string TCPCONN = "TCPCONN";
const std::string TCPCONN_ERR = TCPCONN + sigmaos::util::log::ERR;

class Conn : public sigmaos::io::conn::Conn {
  public:
  // Create a tcp server
  Conn() : _addr({0}) {
    int sockfd;
    log(TCPCONN, "New tcp connection");
    sockfd = socket(AF_INET, SOCK_STREAM, 0);
    if (sockfd == -1) {
      throw std::runtime_error("Failed to create TCP server socket");
    }
    _addr.sin_family = AF_INET;
    _addr.sin_addr.s_addr = htonl(INADDR_ANY);
    _addr.sin_port = 0;
    if (bind(sockfd, (struct sockaddr *) &_addr, sizeof(_addr))) {
      throw std::runtime_error("Bind failed");
    }
    log(TCPCONN, "Bound socket addr");
    if (listen(sockfd, SOCK_BACKLOG)) {
      throw std::runtime_error("Listen failed");
    }
    socklen_t addr_len = sizeof(_addr);
    if (getsockname(sockfd, (sockaddr *) &_addr, &addr_len)) {
      throw std::runtime_error("getsockname");
    }
    log(TCPCONN, "sock addr: {}:{}", _addr.sin_addr.s_addr, _addr.sin_port);
    set_sockfd(sockfd);
    // TODO: accept & handle connections
  }

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

  ~Conn() {}

  private:
  sockaddr_in _addr;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<int, sigmaos::serr::Error> read_bytes(char *b, size_t size);
  std::expected<int, sigmaos::serr::Error> write_bytes(const char *b, size_t size);
};

};
};
