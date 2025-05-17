#pragma once

#include <memory>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/transport/transport.h>
#include <io/transport/call.h>
#include <io/demux/internal/callmap.h>

namespace sigmaos {
namespace io::demux {

const std::string DEMUXSRV = "DEMUXSRV";
const std::string DEMUXSRV_ERR = DEMUXSRV + sigmaos::util::log::ERR;

typedef std::function<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>(std::shared_ptr<sigmaos::io::transport::Call>)> RequestHandler;

class Srv {
  public:
  Srv(std::shared_ptr<sigmaos::io::transport::Transport> trans, RequestHandler serve_request) : _mu(), _closed(false), _trans(trans), _serve_request(serve_request), _callmap(), _reader_thread(std::thread(&Srv::read_requests, this)) {
    log(DEMUXSRV, "New demuxsrv");
  }

  ~Srv() { Close(); }

  std::expected<int, sigmaos::serr::Error> Close();
  bool IsClosed();

  private:
  std::mutex _mu;
  bool _closed;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
  RequestHandler _serve_request;
  sigmaos::io::demux::internal::CallMap _callmap;
  std::thread _reader_thread;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void read_requests();
  void handle_request(std::shared_ptr<sigmaos::io::transport::Call> req);
};

};
};
