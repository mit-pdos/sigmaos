#pragma once

#include <iostream>
#include <memory>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET

class Clnt {
  public:
  Clnt() {
    std::cout << "New sigmap proxy clnt" << std::endl;
    sockfd = socket(AF_UNIX, SOCK_STREAM, 0);
    if (sockfd == -1) {
      throw std::runtime_error("Failed to create spproxy socket fd");
    }
    addr.sun_family = AF_UNIX;
    strncpy(addr.sun_path, SPPROXY_SOCKET.c_str(), sizeof(addr.sun_path) - 1);
    addr.sun_path[sizeof(addr.sun_path) - 1] = '\0';
    if (connect(sockfd, (struct sockaddr*)&addr, sizeof(addr)) == -1) {
      close(sockfd);
      throw std::runtime_error("Failed to connect to spproxy socket");
    }
  }

  ~Clnt() {
    std::cout << "Closing sigmap proxy clnt" << std::endl;
  }

  private:
  int sockfd;
  sockaddr_un addr;
};

};
};
