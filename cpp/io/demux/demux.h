#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>
#include <future>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/transport/transport.h>
#include <io/transport/call.h>
#include <io/demux/internal/callmap.h>

namespace sigmaos {
namespace io::demux {

const std::string DEMUXCLNT = "DEMUXCLNT";
const std::string DEMUXCLNT_ERR = DEMUXCLNT + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::io::transport::Transport> trans) : _mu(), _trans(trans), _callmap(), _reader_thread(std::thread(&Clnt::read_responses, this)) {
    log(DEMUXCLNT, "New demux clnt");
  }

  ~Clnt() { Close(); }

  std::expected<std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error> SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call);
  std::expected<int, sigmaos::serr::Error> Close();
  bool IsClosed();

  private:
  std::mutex _mu;
  std::shared_ptr<sigmaos::io::transport::Transport> _trans;
  sigmaos::io::demux::internal::CallMap _callmap;
  std::thread _reader_thread;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  void read_responses();
};

};
};
