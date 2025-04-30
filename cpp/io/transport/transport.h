#pragma once

#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <io/conn/conn.h>
#include <io/transport/call.h>
#include <io/transport/callmap.h>

namespace sigmaos {
namespace io::transport {

class Transport {
  public:
  Transport(std::shared_ptr<sigmaos::io::conn::UnixConn> conn) : _conn(conn), _calls() {
    std::cout << "New demux clnt" << std::endl;
  }

  ~Transport() { _conn->Close(); }

  std::expected<int, std::string> WriteCall(std::shared_ptr<Call> call);
  std::expected<std::shared_ptr<Call>, std::string> ReadCall();
  std::expected<int, std::string> Close();

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
  sigmaos::io::transport::internal::CallMap _calls;
};

};
};
