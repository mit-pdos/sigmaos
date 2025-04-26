#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <io/conn/conn.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET_PN = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET

class Clnt {
  public:
  Clnt() {
    std::cout << "New sigmap proxy clnt" << std::endl;
    _conn = std::make_shared<sigmaos::io::conn::UnixConn>(SPPROXY_SOCKET_PN);
    std::cout << "Established conn to spproxyd" << std::endl;
  }

  ~Clnt() {
    std::cout << "Closing sigmap proxy clnt" << std::endl;
  }

  std::expected<int, std::string> Test();

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
};

};
};
