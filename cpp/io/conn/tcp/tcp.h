#pragma once

#include <sys/socket.h>
#include <netinet/tcp.h>
#include <sys/un.h>
#include <unistd.h>
#include <netinet/in.h>
#include <arpa/inet.h>

#include <iostream>
#include <memory>
#include <vector>
#include <expected>
#include <format>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <serr/serr.h>

namespace sigmaos {
namespace io::conn::tcpconn {

const int SOCK_BACKLOG = 5;

const std::string TCPCONN = "TCPCONN";
const std::string TCPCONN_ERR = TCPCONN + sigmaos::util::log::ERR;

void set_tcp_nodelay(int sockfd);

class Conn : public sigmaos::io::conn::Conn {
  public:
  // Create a tcp connection
  Conn(std::string id) : sigmaos::io::conn::Conn(id), _addr({0}) {}
  Conn(std::string id, int sockfd, struct sockaddr_in addr) : sigmaos::io::conn::Conn(id, sockfd), _addr(addr) {}
  ~Conn() {}

  protected:
  void init(int sockfd, sockaddr_in addr) {
    _addr = addr;
    sigmaos::io::conn::Conn::init(sockfd);
    set_tcp_nodelay(sockfd);
  }

  private:
  sockaddr_in _addr;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<int, sigmaos::serr::Error> read_bytes(char *b, size_t size);
  std::expected<int, sigmaos::serr::Error> write_bytes(const char *b, size_t size);
};

class ClntConn : public Conn {
  public:
  ClntConn(std::string id, std::string srv_addr, int port) : Conn(id) {
    int sockfd;
    sockaddr_in addr;
    log(TCPCONN, "New tcp client connection {}:{}", srv_addr, port);
    sockfd = socket(AF_INET, SOCK_STREAM, 0);
    if (sockfd == -1) {
      log(TCPCONN_ERR, "Failed to create client TCP socket fd", srv_addr, port);
      fatal("Failed to create client TCP socket fd");
    }
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = inet_addr(srv_addr.c_str());
    addr.sin_port = htons(port);
    if (connect(sockfd, (struct sockaddr *) &addr, sizeof(addr)) == -1) {
      close(sockfd);
      log(TCPCONN_ERR, "Failed to connect client TCP socket", srv_addr, port);
      fatal("Failed to connect client TCP socket");
    }
    init(sockfd, addr);
  }
  ~ClntConn();

  private:
};

class Listener {
  public:
  Listener(std::string id) : _id(id) {
    log(TCPCONN, "New TCP listener");
    _sockfd = socket(AF_INET, SOCK_STREAM, 0);
    if (_sockfd == -1) {
      fatal("failed to create TCP listener socket");
    }
    _addr.sin_family = AF_INET;
    _addr.sin_addr.s_addr = htonl(INADDR_ANY);
    _addr.sin_port = 0;
    if (bind(_sockfd, (struct sockaddr *) &_addr, sizeof(_addr))) {
      fatal("bind failed");
    }
    log(TCPCONN, "Bound socket addr");
    if (listen(_sockfd, SOCK_BACKLOG)) {
      fatal("listen failed");
    }
    socklen_t addr_len = sizeof(_addr);
    if (getsockname(_sockfd, (struct sockaddr *) &_addr, &addr_len)) {
      fatal("getsockname failed");
    }
    log(TCPCONN, "Listener addr: {}:{}", _addr.sin_addr.s_addr, htons(_addr.sin_port));
  }
  ~Listener() { Close(); }

  std::expected<std::shared_ptr<Conn>, sigmaos::serr::Error> Accept();
  std::expected<int, sigmaos::serr::Error> Close();
  int GetPort() { return htons(_addr.sin_port); }

  private:
  std::string _id;
  int _sockfd;
  struct sockaddr_in _addr;
};

};
};
