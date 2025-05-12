#include <io/net/srv.h>

namespace sigmaos {
namespace io::net {

bool Srv::_l = sigmaos::util::log::init_logger(NETSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(NETSRV_ERR);

void Srv::handle_connection(std::shared_ptr<sigmaos::io::conn::Conn> conn) {
  log(NETSRV, "Handling connection!");
}

void Srv::handle_connections() {
  while(!_done) {
    auto res = _lis->Accept();
    if (!res.has_value()) {
      throw std::runtime_error(std::format("Error accept TCP connection: {}", res.error().String()));
    }
    auto conn = res.value();
    log(NETSRV, "Accepted connection");
    // TODO: use thread pool
    std::thread(&Srv::handle_connection, this, conn);
  }
}

};
};
