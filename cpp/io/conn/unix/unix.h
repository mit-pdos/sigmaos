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
  Conn(std::string pn) {
    int sockfd;
    log(UNIXCONN, "New unix connection {}", pn);
    sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (sockfd == -1) {
      throw std::runtime_error("Failed to create spproxy socket fd");
    }
    _addr.sun_family = AF_UNIX;
    strncpy(_addr.sun_path, pn.c_str(), sizeof(_addr.sun_path) - 1);
    _addr.sun_path[sizeof(_addr.sun_path) - 1] = '\0';
    if (connect(sockfd, (struct sockaddr *) &_addr, sizeof(_addr)) == -1) {
      close(sockfd);
      throw std::runtime_error("Failed to connect to spproxy socket");
    }
    set_sockfd(sockfd);
  }

  ~Conn() {}

  private:
  sockaddr_un _addr;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};
};
