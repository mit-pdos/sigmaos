#include <rpc/clnt.h>

#include <rpc/blob.h>

namespace sigmaos {
namespace rpc {

bool Clnt::_l = sigmaos::util::log::init_logger(RPCCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(RPCCLNT_ERR);

std::expected<int, sigmaos::serr::Error> Clnt::RPC(std::string method, google::protobuf::Message &req, google::protobuf::Message &rep) {
  // Create an IOV for RPC inputs
  auto in_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
  auto req_data = std::make_shared<std::string>();
  // Serialize the request and append it to the IOV
  req.SerializeToString(req_data.get());
  in_iov->AppendBuffer(std::make_shared<sigmaos::io::iovec::Buffer>(req_data)); 
  // Extract any input IOVecs from the request RPC
  extract_blob_iov(req, in_iov);
  auto out_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
	// wrapper, and one for the marshaled res proto.Message
  out_iov->AddBuffers(2);
  // Extract any output IOVecs from the response RPC
  extract_blob_iov(rep, out_iov);
  log(RPCCLNT, "out_iov len {}", out_iov->Size());
  // Wrap the RPC and execute it
  auto res = wrap_and_run_rpc(method, in_iov, out_iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  // Get the response
  auto wrapped_rep = res.value();
  // If there was an error in the RPC stack, bail out
  if (wrapped_rep.err().errcode() != sigmaos::serr::TErrNoError) {
    log(RPCCLNT_ERR, "error in RPC stack");
    log(RPCCLNT_ERR, "error in RPC stack error:{}", wrapped_rep.err().ShortDebugString());
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, std::format("rpc error: {}", wrapped_rep.err().ShortDebugString())));
  }
  log(RPCCLNT, "Deserialize reply data");
  // Deserialize the reply
  auto rep_data_buf = out_iov->GetBuffer(0);
  rep.ParseFromString(*rep_data_buf->Get());
  log(RPCCLNT, "Remove reply data buffer");
  // Remove the first element in iov, which contains the serialized reply
  // message
  out_iov->RemoveBuffer(0);
  log(RPCCLNT, "Set blob IOV");
  // Set the reply's blob's IOV to point to the returned data, if applicable.
  set_blob_iov(out_iov, rep);
  log(RPCCLNT, "Done set blob IOV");
  return 0;
}

std::expected<Rep, sigmaos::serr::Error> Clnt::wrap_and_run_rpc(std::string method, const std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov) {
  uint64_t seqno = _seqno++;
  auto wrapped_in_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
  Req req;
  auto wrapper_req_data = std::make_shared<std::string>();
  req.set_method(method);
  // Wrap the request, serialize the wrapper, and write it together with the
  // request data.
  req.SerializeToString(wrapper_req_data.get());
  wrapped_in_iov->AppendBuffer(std::make_shared<sigmaos::io::iovec::Buffer>(wrapper_req_data));
  wrapped_in_iov->Resize(in_iov->Size() + 1);
  for (int i = 0; i < in_iov->Size(); i++) {
    wrapped_in_iov->SetBuffer(i + 1, in_iov->GetBuffer(i));
  }

  // Create the call object to be sent, and perform the RPC.
  auto wrapped_call = std::make_shared<io::transport::Call>(seqno, wrapped_in_iov, out_iov);
  auto res = _chan->SendReceive(wrapped_call);
  if (!res.has_value()) {
    log(RPCCLNT_ERR, "Error sendreceive: {}", res.error().String());
    return std::unexpected(res.error());
  }
  log(RPCCLNT, "Deserialize wrapper for reply seqno: {}", seqno);
  Rep rep;
  // Deserialize the wrapper
  auto wrapper_rep_buf = out_iov->GetBuffer(0);
  rep.ParseFromString(*wrapper_rep_buf->Get());
  log(RPCCLNT, "Remove wrapper buffer for reply seqno: {}", seqno);
  // Remove the wrapper from the out IOVec
  out_iov->RemoveBuffer(0);
  log(RPCCLNT, "Done remove wrapper buffer for reply seqno: {}", seqno);
	// TODO: Record stats
//	rpcc.si.Stat(method, time.Since(start).Microseconds())
  return rep;
}
  
};
};
