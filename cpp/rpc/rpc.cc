#include <rpc/rpc.h>

namespace sigmaos {
namespace rpc {

bool Clnt::_l = sigmaos::util::log::init_logger(RPCCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(RPCCLNT_ERR);

std::pair<Blob *, bool> extract_blob(google::protobuf::Message &msg) {
  Blob *blob = nullptr;
  bool has_blob = false;
  auto r = msg.GetReflection();
  std::vector<const google::protobuf::FieldDescriptor *> fields;
  // List the protobuf's fields
  r->ListFields(msg, &fields);
  for (auto fd : fields) {
    // If one of the fields is a blob, get it & return it
    if (fd->name() == "blob") {
      log(RPCCLNT, "RPC {} has blob: {}", msg.GetTypeName(), fd->full_name());
      google::protobuf::Message *bmsg;
      // Clear the existing blob object if desired, and return ownership of the
      // blob to the caller. Otherwise, return a mutable reference to the blob,
      // still owned by the original message.
      bmsg = r->MutableMessage(&msg, fd);
      log(RPCCLNT, "RPC {} blob type: {}", msg.GetTypeName(), bmsg->GetTypeName());
      blob = dynamic_cast<Blob *>(bmsg);
      log(RPCCLNT, "RPC {} blob type casted, resulting ptr: 0x{:x}", msg.GetTypeName(), (uint64_t) blob);
      has_blob = true;
      break;
    }
  }
  return std::make_pair(blob, has_blob);
}

// If the given RPC has a blob field, extract its IOVecs.
void extract_blob_iov(google::protobuf::Message &msg, std::shared_ptr<sigmaos::io::iovec::IOVec> dst) {
  auto p = extract_blob(msg);
  Blob *blob = p.first;
  bool has_blob = p.second;
  // Bail out if no blob was found
  if (!has_blob) {
    return;
  }
  // Extract input buffers to the in_iov
  auto blob_iov = blob->mutable_iov();
  std::vector<std::string *> extracted_bufs(blob_iov->size());
  blob_iov->ExtractSubrange(0, blob_iov->size(), &(extracted_bufs.front()));
  int prevsz = dst->Size();
  dst->Resize(prevsz + extracted_bufs.size());
  for (int i = 0; i < extracted_bufs.size(); i++) {
    log(RPCCLNT, "Extracting buf starting at 0x{:x}", (uint64_t) &(extracted_bufs.at(i)->front()));
    dst->SetBuffer(prevsz + i, std::make_shared<sigmaos::io::iovec::Buffer>(extracted_bufs.at(i)));
  }
  // Clear the extracted buffer vector so that it doesn't free the underlying
  // buffer memory when it is deleted. The memory is now owned by dst.
  extracted_bufs.clear();
}

// If the given RPC has a blob field, extract its IOVecs.
void set_blob_iov(std::shared_ptr<sigmaos::io::iovec::IOVec> src, google::protobuf::Message &msg) {
  auto p = extract_blob(msg);
  Blob *blob = p.first;
  bool has_blob = p.second;
  // Bail out if no blob was found
  if (!has_blob) {
    return;
  }
  // Get a pointer to the blob IOV
  auto blob_iov = blob->mutable_iov();
  for (int i = 0; i < src->Size(); i++) {
    auto src_buf = src->GetBuffer(i);
    // Sanity check: output buffers passed to proto library shouldn't be ref
    // counted at this layer
    if (src_buf->IsRefCounted()) {
      throw std::runtime_error("Ref counted buffer moved to protobuf");
    }
    blob_iov->AddAllocated(src_buf->Get());
  }
}

std::expected<int, sigmaos::serr::Error> Clnt::RPC(std::string method, google::protobuf::Message &req, google::protobuf::Message &rep) {
  log(RPCCLNT, "Get in_blob {}", method);
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
  log(RPCCLNT, "Get out_blob {}", method);
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
  if (wrapped_rep.err().errcode() != 0) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, std::format("rpc error: {}", wrapped_rep.err().ShortDebugString())));
  }
  // Deserialize the reply
  auto rep_data_buf = out_iov->GetBuffer(0);
  rep.ParseFromString(*rep_data_buf->Get());
  // Remove the first element in iov, which contains the serialized reply
  // message
  out_iov->RemoveBuffer(0);
  // Set the reply's blob's IOV to point to the returned data, if applicable.
  set_blob_iov(out_iov, rep);
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
  // TODO: serialize directly to ostream
  req.SerializeToString(wrapper_req_data.get());
  wrapped_in_iov->AppendBuffer(std::make_shared<sigmaos::io::iovec::Buffer>(wrapper_req_data));
  wrapped_in_iov->Resize(in_iov->Size() + 1);
  for (int i = 0; i < in_iov->Size(); i++) {
    wrapped_in_iov->SetBuffer(i + 1, in_iov->GetBuffer(i));
  }

  // Create the call object to be sent, and perform the RPC.
  auto wrapped_call = std::make_shared<io::transport::Call>(seqno, wrapped_in_iov, out_iov);
  auto res = _demux->SendReceive(wrapped_call);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  Rep rep;
  // Deserialize the wrapper
  auto wrapper_rep_buf = out_iov->GetBuffer(0);
  // TODO: deserialize directly from stream
  rep.ParseFromString(*wrapper_rep_buf->Get());
  // Remove the wrapper from the out IOVec
  out_iov->RemoveBuffer(0);
	// TODO: Record stats
//	rpcc.si.Stat(method, time.Since(start).Microseconds())
  return rep;
}
  
};
};
