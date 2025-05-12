#include <io/demux/srv.h>

namespace sigmaos {
namespace io::demux {

bool Srv::_l = sigmaos::util::log::init_logger(DEMUXSRV);
bool Srv::_l_e = sigmaos::util::log::init_logger(DEMUXSRV_ERR);

void Srv::read_requests() {
  while (true) {
    if (IsClosed()) {
      log(DEMUXSRV, "demuxsrv closed, reader thread exiting");
      break;
    }
    auto res = _trans->ReadCall();
    if (!res.has_value()) {
      log(DEMUXSRV_ERR, "demuxsrv read call error: {}", res.error().String());
      // TODO: report error
      break;
    }
    // TODO: handle request in a thread pool
    std::thread(&Srv::handle_request, this, res.value());
  }
}

void Srv::handle_request(std::shared_ptr<sigmaos::io::transport::Call> req) {
  auto rep = _serve_request(req);
  if (!rep.has_value()) {
    log(DEMUXSRV_ERR, "demuxsrv serve_request error: {}", rep.error().String());
    return;
  }
  std::lock_guard<std::mutex> guard(_mu);
  auto res = _trans->WriteCall(rep.value());
  if (!res.has_value()) {
    log(DEMUXSRV_ERR, "demuxsrv WriteCall error: {}", res.error().String());
  }
}

std::expected<int, sigmaos::serr::Error> Srv::Close() {
  log(DEMUXSRV, "Close demuxsrv");
  std::lock_guard<std::mutex> guard(_mu);
  _closed = true;
  return 0;
}

bool Srv::IsClosed() {
  std::lock_guard<std::mutex> guard(_mu);
  return _closed;
}

};
};
