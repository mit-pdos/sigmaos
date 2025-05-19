#include <io/net/srv.h>

namespace sigmaos {
namespace io::net {

bool Srv::_l = sigmaos::util::log::init_logger(NETSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(NETSRV_ERR);

void Srv::handle_connection(std::shared_ptr<sigmaos::io::conn::Conn> conn) {
  log(NETSRV, "Handling connection!");
  // TODO: Make transport
  auto trans = std::make_shared<sigmaos::io::transport::Transport>(conn);
  log(NETSRV, "Made transport");
  _demux_srvs.push_back(std::make_shared<sigmaos::io::demux::Srv>(trans, _serve_request, _demux_init_nthread));
  log(NETSRV, "Made demuxsrv");
}

void Srv::handle_connections() {
  while(!_done) {
    auto res = _lis->Accept();
    if (!res.has_value()) {
      fatal("Error accept TCP connection: {}", res.error().String());
    }
    auto conn = res.value();
    log(NETSRV, "Accepted connection");
    // Handle the connection via the thread pool
    _thread_pool.Run(std::bind(&Srv::handle_connection, this, conn));
  }
}

};
};
