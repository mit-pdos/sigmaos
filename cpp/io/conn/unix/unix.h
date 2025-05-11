#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <vector>
#include <expected>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <serr/serr.h>

namespace sigmaos {
namespace io::conn::unixconn {

const std::string UNIXCONN = "UNIXCONN";
const std::string UNIXCONN_ERR = UNIXCONN + sigmaos::util::log::ERR;

class Conn : public sigmaos::io::conn::Conn {
  public:
  // Create a unix socket connection
  Conn() : sigmaos::io::conn::Conn(), _addr({0}) {}
  ~Conn() {}

  protected:
  void init(int sockfd, sockaddr_un addr) {
    _addr = addr;
    sigmaos::io::conn::Conn::init(sockfd);
  }

  private:
  sockaddr_un _addr;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

class ClntConn : public Conn {
  public:
  ClntConn(std::string pn) {
    int sockfd;
    sockaddr_un addr;
    log(UNIXCONN, "New unix client connection {}", pn);
    sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (sockfd == -1) {
      throw std::runtime_error("Failed to create spproxy socket fd");
    }
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, pn.c_str(), sizeof(addr.sun_path) - 1);
    addr.sun_path[sizeof(addr.sun_path) - 1] = '\0';
    if (connect(sockfd, (struct sockaddr *) &addr, sizeof(addr)) == -1) {
      close(sockfd);
      throw std::runtime_error("Failed to connect to spproxy socket");
    }
    init(sockfd, addr);
  }
  ~ClntConn() {}
  private:
};

};
};
