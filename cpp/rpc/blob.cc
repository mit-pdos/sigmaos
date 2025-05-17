#include <rpc/blob.h>

namespace sigmaos {
namespace rpc {

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
      blob = dynamic_cast<Blob *>(bmsg);
      has_blob = true;
      break;
    }
  }
  return std::make_pair(blob, has_blob);
}

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

};
};
