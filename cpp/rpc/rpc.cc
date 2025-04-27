#include <rpc/rpc.h>

namespace sigmaos {
namespace rpc {


std::expected<int, std::string> Clnt::RPC(std::string method, const google::protobuf::Message &req, google::protobuf::Message &rep) {
  // TODO: get blob
//	inblob := rpc.GetBlob(arg)
//	var iniov sessp.IoVec
//	if inblob != nil {
//		iniov = inblob.GetIoVec()
//		inblob.SetIoVec(nil)
//	}
  std::string req_data;
  std::string rep_data;
  Rep wrapped_rep;
  // Serialize the request
  // TODO: serialize directly to ostream
  req.SerializeToString(&req_data);
  std::vector<std::vector<unsigned char>> in_iov;
  in_iov.push_back(std::vector<unsigned char>(req_data.begin(), req_data.end()));
  // TODO: handle iovec output buffers
//	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
//	// wrapper, and one for the marshaled res proto.Message
//	outiov := make(sessp.IoVec, 2)
//	outblob := rpc.GetBlob(res)
//	if outblob != nil { // handle blob
//		// Get the reply's blob, if it has one, so that data can be read directly
//		// into buffers in its IoVec
//		outiov = append(outiov, outblob.GetIoVec()...)
//	}
  std::vector<std::vector<unsigned char>> out_iov;
  auto res = wrap_and_run_rpc(method, in_iov, out_iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  wrapped_rep = res.value();
  if (wrapped_rep.err().errcode() != 0) {
    return std::unexpected(std::format("rpc error: {}", wrapped_rep.err().DebugString()));
  }
  // Deserialize the reply
  rep_data = std::string(out_iov[1].begin(), out_iov[1].end());
  // TODO: deserialize directly from stream (should we be using strings at all?)
  rep.ParseFromString(rep_data);
  // TODO: handle blobs
//	if outblob != nil {
//		// Need to get the blob again, because its value will be reset during
//		// unmarshaling
//		outblob = rpc.GetBlob(res)
//		// Set the IoVec to handle replies with blobs
//		outblob.SetIoVec(outiov[2:])
//	}
//	return nil
//
  return 0;
}

std::expected<Rep, std::string> Clnt::wrap_and_run_rpc(std::string method, const std::vector<std::vector<unsigned char>> &in_iov, std::vector<std::vector<unsigned char>> &out_iov) {
  // TODO: atomically choose a seqno
  uint64_t seqno = _seqno++;
  std::vector<std::vector<unsigned char>> wrapped_in_iov;
  Req req;
  std::string wrapper_req_data;
  std::vector<std::vector<unsigned char>> wrapped_out_iov;
  // TODO: make ptr?
  Rep rep;
  std::string wrapper_rep_data;

  req.set_method(method);
  // Wrap the request, serialize the wrapper, and write it together with the
  // request data.
  // TODO: serialize directly to ostream
  req.SerializeToString(&wrapper_req_data);
  wrapped_in_iov.push_back(std::vector<unsigned char>(wrapper_req_data.begin(), wrapper_req_data.end()));
  wrapped_in_iov.resize(in_iov.size() + 1);
  wrapped_in_iov.insert(std::next(wrapped_in_iov.begin()), in_iov.begin(), in_iov.end());

  // Create the call object to be sent, and perform the RPC.
  io::transport::Call wrapped_call(seqno, wrapped_in_iov);
  auto res = _demux->SendReceive(wrapped_call, out_iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  wrapped_out_iov = res.value()->GetIOVec();
  // Deserialize the wrapper
  wrapper_rep_data = std::string(wrapped_out_iov[0].begin(), wrapped_out_iov[0].end());
  // TODO: deserialize directly from stream (should we be using strings at all?)
  rep.ParseFromString(wrapper_rep_data);
	// TODO: Record stats
//	rpcc.si.Stat(method, time.Since(start).Microseconds())
  return rep;
}
  
};
};
