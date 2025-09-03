#include <io/demux/clnt.h>

namespace sigmaos {
namespace io::demux {

bool Clnt::_l = sigmaos::util::log::init_logger(DEMUXCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(DEMUXCLNT_ERR);

std::expected<std::shared_ptr<sigmaos::io::transport::Call>,
              sigmaos::serr::Error>
Clnt::SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call) {
  // Create a promise
  auto p = std::make_unique<std::promise<std::expected<
      std::shared_ptr<sigmaos::io::transport::Call>, sigmaos::serr::Error>>>();
  // Get the corresponding future
  auto f = p->get_future();
  {
    // Add it to the call map
    auto res = _callmap.Put(call->GetSeqno(), std::move(p));
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
  }
  {
    // Take a lock so that writes are atomic
    std::lock_guard<std::mutex> guard(_mu);
    auto res = _trans->WriteCall(call);
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
  }
  // Wait for the reader thread to materialize the response
  f.wait();
  log(DEMUXCLNT, "Got response seqno {}", call->GetSeqno());
  // Get and return the result
  return f.get();
}

std::expected<int, sigmaos::serr::Error> Clnt::Close() {
  log(DEMUXCLNT, "Close");
  if (_callmap.IsClosed()) {
    log(DEMUXCLNT, "Close: already closed");
    return 0;
  }
  {
    auto res = _trans->Close();
    if (!res.has_value()) {
      log(DEMUXCLNT_ERR, "Err close trans: {}", res.error());
    }
  }
  log(DEMUXCLNT, "Close callmap");
  _callmap.Close();
  log(DEMUXCLNT, "Done closing callmap");
  // Join the reader thread
  log(DEMUXCLNT, "Join reader thread");
  // TODO: join the reader thread. In order to do so, we need to switch the
  // underlying connection's Read syscalls to polling, so that the reader
  // thread can find out we are closing the connection and safely close the
  // underlying connection FD.
  // _reader_thread.join();
  log(DEMUXCLNT, "Done join reader thread");
  log(DEMUXCLNT, "Done Close");
  return 0;
}

bool Clnt::IsClosed() { return _callmap.IsClosed(); }

void Clnt::read_responses() {
  while (true) {
    // Read a response
    auto res = _trans->ReadCall();
    if (!res.has_value()) {
      log(DEMUXCLNT_ERR, "Err reader: {}", res.error());
      _callmap.Close();
      return;
    }
    auto call = res.value();
    {
      auto res = _callmap.Remove(call->GetSeqno());
      if (res.has_value()) {
        auto p = std::move(res.value());
        p->set_value(call);
      } else {
        fatal("reply with no matching req");
      }
    }
  }
}

};  // namespace io::demux
};  // namespace sigmaos
