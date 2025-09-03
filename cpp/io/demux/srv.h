#pragma once

#include <io/demux/internal/callmap.h>
#include <io/transport/call.h>
#include <io/transport/transport.h>
#include <serr/serr.h>
#include <threadpool/threadpool.h>
#include <util/log/log.h>

#include <memory>

namespace sigmaos {
namespace io::demux {

const std::string DEMUXSRV = "DEMUXSRV";
const std::string DEMUXSRV_ERR = DEMUXSRV + sigmaos::util::log::ERR;

typedef std::function<std::expected<
    std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>(
    std::shared_ptr<sigmaos::io::transport::Call>)>
    RequestHandler;

class Srv {
 public:
  Srv(std::shared_ptr<sigmaos::io::transport::Transport> trans,
      RequestHandler serve_request)
      : Srv(trans, serve_request, 0) {}

  Srv(std::shared_ptr<sigmaos::io::transport::Transport> trans,
      RequestHandler serve_request, int init_nthread)
      : _mu(),
        _closed(false),
        _trans(trans),
        _serve_request(serve_request),
        _callmap(),
        _reader_thread(std::thread(&Srv::read_requests, this)),
        _thread_pool("demuxsrv", init_nthread) {
    log(DEMUXSRV, "New demuxsrv init_nthread={}", init_nthread);
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
  sigmaos::threadpool::Threadpool _thread_pool;
  std::thread _reader_thread;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void read_requests();
  void handle_request(std::shared_ptr<sigmaos::io::transport::Call> req);
};

};  // namespace io::demux
};  // namespace sigmaos
