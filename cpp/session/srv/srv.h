#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>

#include <util/log/log.h>
#include <io/net/srv.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <serr/serr.h>
#include <sigmap/sigmap.pb.h>
#include <sigmap/const.h>

namespace sigmaos {
namespace session::srv {

const std::string SESSSRV = "SESSSRV";
const std::string SESSSRV_ERR = SESSSRV + sigmaos::util::log::ERR;

class Srv {
  public:
  Srv() : _done(false), _sessions() {
    log(SESSSRV, "Starting net server");
    _netsrv = std::make_shared<sigmaos::io::net::Srv>(std::bind(&Srv::serve_request, this, std::placeholders::_1));
    int port = _netsrv->GetPort();
    log(SESSSRV, "Net server started with port {}", port);
  }
  ~Srv() {}

  std::shared_ptr<TendpointProto> GetEndpoint();
  std::expected<int, sigmaos::serr::Error> Close() {
    _done = true;
    return _netsrv->Close();
  }
  private:
  bool _done;
  std::shared_ptr<sigmaos::io::net::Srv> _netsrv;
  std::vector<std::shared_ptr<sigmaos::io::demux::Srv>> _sessions;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
  
  // TODO: move sesssrv request handler typedef to its own header
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> serve_request(std::shared_ptr<sigmaos::io::transport::Call> req);
};

};
};
