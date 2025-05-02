#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/demux/demux.h>
#include <serr/serr.h>
#include <rpc/rpc.h>
#include <proc/proc.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET_PN = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET
const std::string SPPROXYCLNT = "SPPROXYCLNT";
const std::string SPPROXYCLNT_ERR = "SPPROXYCLNT" + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt() {
    _env = sigmaos::proc::GetProcEnv();
    log(SPPROXYCLNT, "New clnt {}", _env->String());
    _conn = std::make_shared<sigmaos::io::conn::UnixConn>(SPPROXY_SOCKET_PN);
    _trans = std::make_shared<sigmaos::io::transport::Transport>(_conn);
    _demux = std::make_shared<sigmaos::io::demux::Clnt>(_trans);
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(_demux);
    log(SPPROXYCLNT, "Initializing proxy conn");
    // Initialize the sigmaproxyd connection
    init_conn();
  }

  ~Clnt() { Close(); }

  std::expected<int, sigmaos::serr::Error> Test();
  void Close() { 
    log(SPPROXYCLNT, "Close");
    _rpcc->Close(); 
    log(SPPROXYCLNT, "Done close");
  }

  // Stubs
  // TODO

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  std::shared_ptr<sigmaos::proc::ProcEnv> _env;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void init_conn();
};

};
};
