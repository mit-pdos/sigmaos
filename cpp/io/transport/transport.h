#pragma once

#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <io/transport/call.h>
#include <io/transport/internal/callmap.h>

namespace sigmaos {
namespace io::transport {

const std::string TRANSPORT = "TRANSPORT";
const std::string TRANSPORT_ERR = "TRANSPORT" + sigmaos::util::log::ERR;

class Transport {
  public:
  Transport(std::shared_ptr<sigmaos::io::conn::UnixConn> conn) : _conn(conn), _calls() {
    log(TRANSPORT, "New transport");
  }

  ~Transport() { _conn->Close(); }

  std::expected<int, std::string> WriteCall(std::shared_ptr<Call> call);
  std::expected<std::shared_ptr<Call>, std::string> ReadCall();
  std::expected<int, std::string> Close();

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
  sigmaos::io::transport::internal::CallMap _calls;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};
};
