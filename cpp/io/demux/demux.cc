#include <io/demux/demux.h>

namespace sigmaos {
namespace io::demux {

std::expected<std::shared_ptr<sigmaos::io::transport::Call>, std::string> Clnt::SendReceive(const sigmaos::io::transport::Call &call, std::vector<std::vector<unsigned char>> &outiov) {
// TODO: make a channel & add it to a callmap (with the appropriate tag)
//	ch := make(chan reply)
//	if err := dmx.callmap.put(req.Tag(), ch); err != nil {
//		db.DPrintf(db.DEMUXCLNT_ERR, "SendReceive: enqueue req %v err %v", req, err)
//		return nil, err
//	}
// TODO: add the out iovec to the map
//	if err := dmx.iovm.Put(req.Tag(), outiov); err != nil {
//		db.DPrintf(db.DEMUXCLNT_ERR, "SendReceive: iovm enqueue req %v err %v", req, err)
//		return nil, err
//	}
// TODO: lock & unlock
//	dmx.mu.Lock()
  // TODO: convert request to callI
  std::cout << "wrapped in iov demxuclnt.SendReceive call 1 len " << call.GetIOVec().size() << " call ptr " << &call << std::endl;
  {
    std::cout << "wrapped in iov demxuclnt.SendReceive call 2 len " << call.GetIOVec().size() << std::endl;
  	auto res = _trans->WriteCall(call);
  //	dmx.mu.Unlock()
    if (!res.has_value()) {
      return std::unexpected(res.error());
    }
  }
  return _trans->ReadCall(outiov);
  
  // TODO: wait for reader to return a result via the channel
	// Listen to the reply channel regardless of error status, so the reader
	// thread doesn't block indefinitely trying to deliver the "TErrUnreachable"
	// reply.
// 	rep := <-ch
//	return rep.rep, rep.err
}

std::expected<int, std::string> Clnt::Close() {
  throw std::runtime_error("unimplmented");
}

bool Clnt::IsClosed() {
  throw std::runtime_error("unimplmented");
}

};
};
