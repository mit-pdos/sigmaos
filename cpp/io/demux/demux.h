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
  Clnt(std::shared_ptr<sigmaos::io::transport::Transport> trans) : _trans(trans) {
    std::cout << "New demux clnt" << std::endl;
  // TODO: start reader thread
  }

  ~Clnt() {
    std::cout << "Close demux clnt" << std::endl;
    Close();
  }

  // TODO: Call type?
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string> SendReceive(const sigmaos::io::transport::Call &call, std::vector<std::vector<unsigned char>> outiov);
  std::expected<int, std::string> Close();
  bool IsClosed();

  private:
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
};

};
};
