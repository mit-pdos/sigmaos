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
    log(DEMUXSRV, "demuxsrv done reading call");
    auto req = res.value();
    // Swap input/output IOV, because transport package is written from the
    // client perspective.
    req->SwapIOVecs();
    log(DEMUXSRV, "demuxsrv done reading call {}, handle request", req->GetSeqno());
    _thread_pool.Run(std::bind(&Srv::handle_request, this, res.value()));
  }
}

void Srv::handle_request(std::shared_ptr<sigmaos::io::transport::Call> req) {
  log(DEMUXSRV, "demuxsrv handle request seqno: {}", req->GetSeqno());
  std::shared_ptr<sigmaos::io::transport::Call> rep;
  {
    // Serve the request
    auto res = _serve_request(req);
    if (!res.has_value()) {
      log(DEMUXSRV_ERR, "demuxsrv serve_request error: {}", res.error().String());
      fatal("demuxsrv serve_request unimplemented: {}", res.error().String());
      // TODO: should we return here?
      return;
    }
    rep = res.value();
  }
  {
    // Swap input/output IOV, because transport package is written from the
    // client perspective.
    rep->SwapIOVecs();
    // Write the reply
    std::lock_guard<std::mutex> guard(_mu);
    auto res = _trans->WriteCall(rep);
    if (!res.has_value()) {
      log(DEMUXSRV_ERR, "demuxsrv WriteCall error: {}", res.error().String());
    }
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
