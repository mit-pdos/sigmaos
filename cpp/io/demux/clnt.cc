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
  std::call_once(_close_once, [this]() {
    // Shutdown the transport to unblock any read() call in the reader thread.
    // shutdown() signals EOF without closing the FD, so there is no race.
    auto res = _trans->Shutdown();
    if (!res.has_value()) {
      log(DEMUXCLNT_ERR, "Err shutdown trans: {}", res.error());
    }
    // Join the reader thread. It will exit because shutdown causes read() to
    // return EOF or an error. Only after the join is the FD safe to close.
    log(DEMUXCLNT, "Join reader thread");
    _reader_thread.join();
    log(DEMUXCLNT, "Done join reader thread");
    // Close the FD now that no thread is reading from it.
    auto close_res = _trans->Close();
    if (!close_res.has_value()) {
      log(DEMUXCLNT_ERR, "Err close trans: {}", close_res.error());
    }
    log(DEMUXCLNT, "Close callmap");
    _callmap.Close();
    log(DEMUXCLNT, "Done closing callmap");
    log(DEMUXCLNT, "Done Close");
  });
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
