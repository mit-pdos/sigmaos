#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <io/transport/transport.h>

namespace sigmaos {
namespace io::demux {

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::io::transport::Transport> trans) : trans(trans) {
    std::cout << "New demux clnt" << std::endl;
  }

  ~Clnt() {
    std::cout << "Close demux clnt" << std::endl;
    Close();
  }

  // TODO: Call type?
  std::expected<bool, std::string> SendReceive(const void *call, std::vector<std::vector<unsigned char>> outiov);
  std::expected<bool, std::string> Close();
  bool IsClosed();

  private:
  std::shared_ptr<sigmaos::io::transport::Transport> trans;
};

};
};
