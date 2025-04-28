#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/demux/demux.h>
#include <rpc/rpc.h>
#include <proc/proc.h>

namespace sigmaos {
namespace proxy::sigmap {

const std::string SPPROXY_SOCKET_PN = "/tmp/spproxyd/spproxyd.sock"; // sigmap/sigmap.go SIGMASOCKET

class Clnt {
  public:
  Clnt() {
    _env = sigmaos::proc::GetProcEnv();
    std::cout << "New sigmap proxy clnt " << _env->String() << std::endl;
    _conn = std::make_shared<sigmaos::io::conn::UnixConn>(SPPROXY_SOCKET_PN);
    _trans = std::make_shared<sigmaos::io::transport::Transport>(_conn);
    _demux = std::make_shared<sigmaos::io::demux::Clnt>(_trans);
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(_demux);
    std::cout << "Established conn to spproxyd" << std::endl;
    std::cout << "Initializing conn to spproxyd" << std::endl;
    // Initialize the sigmaproxyd connection
    {
      SigmaInitReq req;
      SigmaErrRep rep;
      // Set the proc env proto
      req.set_allocated_procenvproto(_env->GetProto());
      // Execute the RPC
      auto res = _rpcc->RPC("SPProxySrvAPI.Init", req, rep);
      if (!res.has_value()) {
        std::cout << "Err RPC: " << res.error() << std::endl;
        return res;
      }
      if (rep.err().errcode() != 0) {
        throw std::runtime_error(std::format("init rpc error: {}", rep.err().DebugString()));
      }
      std::cout << "Init RPC successful!" << std::endl;
      // Make sure to release the proc env proto pointer so it isn't destroyed
      req.release_procenvproto();
    }
  }

  ~Clnt() {
    std::cout << "Closing sigmap proxy clnt" << std::endl;
  }

  std::expected<int, std::string> Test();

  private:
  std::shared_ptr<sigmaos::io::conn::UnixConn> _conn;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  std::shared_ptr<sigmaos::proc::ProcEnv> _env;
};

};
};
