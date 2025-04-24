#pragma once

#include <iostream>
#include <memory>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

namespace sigmaos {
namespace io::demux {

class Clnt {
  public:
  Clnt() {
    std::cout << "New demux clnt" << std::endl;
  }

  ~Clnt() {
    std::cout << "Closing demux clnt" << std::endl;
  }

  private:
};

};
};
