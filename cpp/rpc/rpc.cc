#include <rpc/rpc.h>

namespace sigmaos {
namespace rpc {

bool Clnt::_l = sigmaos::util::log::init_logger(RPCCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(RPCCLNT_ERR);

// If the given RPC has a blob field, extract its IOVecs.
void extract_blob_iov(google::protobuf::Message &msg, std::vector<std::string *> *dst) {
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
  // Bail out if no blob was found
  if (!has_blob) {
    return;
  }
  // Extract input buffers to the in_iov
  auto blob_iov = blob->mutable_iov();
  std::vector<std::string *> extracted_bufs(blob_iov->size());
  blob_iov->ExtractSubrange(0, blob_iov->size(), &(extracted_bufs.front()));
  int prevsz = dst->size();
  dst->resize(prevsz + extracted_bufs.size());
  for (int i = 0; i < extracted_bufs.size(); i++) {
    log(RPCCLNT, "Extracting buf starting at 0x{:x}", (uint64_t) &(extracted_bufs.at(i)->front()));
    dst->at(prevsz + i) = extracted_bufs.at(i);
//    dst->at(prevsz + i) = std::vector<unsigned char>((unsigned char *) &(extracted_bufs.at(i)->front()), extracted_bufs.at(i)->size());
  }
  // Clear the extracted buffer vector so that it doesn't free the underlying
  // buffer memory when it is deleted. The memory is now owned by dst.
  extracted_bufs.clear();
}

// If the given RPC has a blob field, extract its IOVecs.
void set_blob_iov(std::vector<std::string *> *src, google::protobuf::Message &msg) {
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
  // Bail out if no blob was found
  if (!has_blob) {
    return;
  }
  // Get a pointer to the blob IOV
  auto blob_iov = blob->mutable_iov();
  // TODO: factor the above into its own fn
  for (auto src_buf : *src) {
    blob_iov->AddAllocated(src_buf);
  }
}

std::expected<int, sigmaos::serr::Error> Clnt::RPC(std::string method, google::protobuf::Message &req, google::protobuf::Message &rep) {
  log(RPCCLNT, "Get in_blob {}", method);
  // Create a vector with a space for the serialized request protobuf
  std::vector<std::string *> in_iov(1);
  // Extract any input IOVecs from the request RPC
  extract_blob_iov(req, &in_iov);
  std::string req_data;
  std::string *rep_data;
  Rep wrapped_rep;
  // Serialize the request
  // TODO: serialize directly to ostream
  req.SerializeToString(&req_data);
  in_iov.at(0) = &req_data; 
	// Prepend 2 empty slots to the out iovec: one for the rpcproto.Rep
	// wrapper, and one for the marshaled res proto.Message
  // TODO: memory leaks?
  std::vector<std::string *> out_iov = {new std::string(), new std::string()};
  log(RPCCLNT, "Get out_blob {}", method);
  // Extract any output IOVecs from the response RPC
  extract_blob_iov(rep, &out_iov);
  log(RPCCLNT, "out_iov len {}", out_iov.size());
  auto res = wrap_and_run_rpc(method, in_iov, out_iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  wrapped_rep = res.value();
  if (wrapped_rep.err().errcode() != 0) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrUnreachable, std::format("rpc error: {}", wrapped_rep.err().ShortDebugString())));
  }
  // Deserialize the reply
  rep_data = out_iov[0];
  // TODO: deserialize directly from stream (should we be using strings at all?)
  rep.ParseFromString(*rep_data);
  // Remove the first element in iov, which contains the serialized reply
  // message
  // TODO: free?
  out_iov.erase(out_iov.begin());
  // Set the reply's blob's IOV to point to the returned data, if applicable.
  set_blob_iov(&out_iov, rep);
// TODO: clear iov?
//  out_iov.clear();
  // Set out blob again
//  Blob *out_blob = GetBlob(rep, false);
  return 0;
}

std::expected<Rep, sigmaos::serr::Error> Clnt::wrap_and_run_rpc(std::string method, const std::vector<std::string *> &in_iov, std::vector<std::string *> &out_iov) {
  uint64_t seqno = _seqno++;
  std::vector<std::string *> wrapped_in_iov;
  Req req;
  std::string wrapper_req_data;
  Rep rep;
  std::string *wrapper_rep_data;

  req.set_method(method);
  // Wrap the request, serialize the wrapper, and write it together with the
  // request data.
  // TODO: serialize directly to ostream
  req.SerializeToString(&wrapper_req_data);
  wrapped_in_iov.push_back(&wrapper_req_data);
  wrapped_in_iov.resize(in_iov.size() + 1);
  for (int i = 0; i < in_iov.size(); i++) {
    wrapped_in_iov.at(i + 1) = in_iov.at(i);
  }

  // Create the call object to be sent, and perform the RPC.
  auto wrapped_call = std::make_shared<io::transport::Call>(seqno, wrapped_in_iov, out_iov);
  auto res = _demux->SendReceive(wrapped_call);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  // Deserialize the wrapper
  wrapper_rep_data = out_iov[0];
  // TODO: deserialize directly from stream
  rep.ParseFromString(*wrapper_rep_data);
  // Remove the wrapper from the out IOVec
  for (int i = 0; i < out_iov.size() - 1; i++) {
    out_iov.at(i) = out_iov.at(i + 1);
  }
  out_iov.resize(out_iov.size() - 1);
	// TODO: Record stats
//	rpcc.si.Stat(method, time.Since(start).Microseconds())
  return rep;
}
  
};
};
