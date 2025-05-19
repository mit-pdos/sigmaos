#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>

#include <util/log/log.h>
#include <io/conn/conn.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <serr/serr.h>
#include <threadpool/threadpool.h>

namespace sigmaos {
namespace io::net {

const std::string NETSRV = "NETSRV";
const std::string NETSRV_ERR = NETSRV + sigmaos::util::log::ERR;

class Srv {
  public:
  Srv(sigmaos::io::demux::RequestHandler serve_request) : Srv(serve_request, 0) {}
  Srv(sigmaos::io::demux::RequestHandler serve_request, int demux_init_nthread) : _done(false), _serve_request(serve_request), _demux_srvs(), _thread_pool("netsrv"), _demux_init_nthread(demux_init_nthread) {
    log(NETSRV, "Starting net server");
    _lis = std::make_shared<sigmaos::io::conn::tcpconn::Listener>();
    log(NETSRV, "TCP server started");
    connection_handler = std::thread(&Srv::handle_connections, this);
  }
  ~Srv() {}

  int GetPort() { return _lis->GetPort(); }
  std::expected<int, sigmaos::serr::Error> Close() {
    _done = true;
    // TODO: join connection-handler thread
    return _lis->Close();
  }
  private:
  bool _done;
  sigmaos::io::demux::RequestHandler _serve_request;
  std::vector<std::shared_ptr<sigmaos::io::demux::Srv>> _demux_srvs;
  sigmaos::threadpool::Threadpool _thread_pool;
  int _demux_init_nthread;
  std::shared_ptr<sigmaos::io::conn::tcpconn::Listener> _lis;
  std::thread connection_handler;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void handle_connection(std::shared_ptr<sigmaos::io::conn::Conn> conn);
  void handle_connections();
};

};
};
