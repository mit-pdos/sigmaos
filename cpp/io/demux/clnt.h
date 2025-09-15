#pragma once

#include <io/demux/internal/callmap.h>
#include <io/transport/call.h>
#include <io/transport/transport.h>
#include <rpc/channel.h>
#include <serr/serr.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>
#include <util/log/log.h>

#include <expected>
#include <future>
#include <iostream>
#include <memory>

namespace sigmaos {
namespace io::demux {

const std::string DEMUXCLNT = "DEMUXCLNT";
const std::string DEMUXCLNT_ERR = DEMUXCLNT + sigmaos::util::log::ERR;

class Clnt : public sigmaos::rpc::Channel {
 public:
  Clnt(std::shared_ptr<sigmaos::io::transport::Transport> trans)
      : _mu(),
        _trans(trans),
        _callmap(),
        _reader_thread(std::thread(&Clnt::read_responses, this)) {
    log(DEMUXCLNT, "New demux clnt");
  }

  ~Clnt() { Close(); }

  std::expected<std::shared_ptr<sigmaos::io::transport::Call>,
                sigmaos::serr::Error>
  SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call);
  std::expected<int, sigmaos::serr::Error> Close();
  bool IsClosed();
  bool IsInitialized() { return true; }
  std::expected<int, sigmaos::serr::Error> Init() { return 0; }

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

};  // namespace io::demux
};  // namespace sigmaos
