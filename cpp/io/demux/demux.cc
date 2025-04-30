#include <io/demux/demux.h>

namespace sigmaos {
namespace io::demux {

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string> Clnt::SendReceive(std::shared_ptr<sigmaos::io::transport::Call> call) {
  // Create a promise
  auto p = std::make_unique<std::promise<std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string>>>();
  // Get the corresponding future
  auto f = p->get_future();
  {
    // Add it to the call map
    auto res = _callmap.Put(call->GetSeqno(), std::move(p));
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
  }
// TODO: lock & unlock
//	dmx.mu.Lock()
  // TODO: take a lock?
  {
  	auto res = _trans->WriteCall(call);
  //	dmx.mu.Unlock()
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
  }
  // Wait for the reader thread to materialize the response
  f.wait();
  // Get and return the result
  return f.get();
}

std::expected<int, std::string> Clnt::Close() {
	if (_callmap.IsClosed()) {
    return 0;
	}
  auto res = _trans->Close();
  if (!res.has_value()) {
    std::cout << "Error DemuxClnt close trans: " << res.error() << std::endl;
  }
  return _callmap.Close();
}

bool Clnt::IsClosed() {
  return _callmap.IsClosed();
}

void Clnt::read_responses() {
  throw std::runtime_error("unimplmented");
  while(true) {
    // Read a response
    auto res = _trans->ReadCall();
    if (!res.has_value()) {
      std::cout << "Error demuxclnt read_responses: " << res.error() << std::endl;
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
        throw std::runtime_error("reply with no matching req");
      }
    }
  }
}

};
};
