#include <rpc/blob.h>
#include <rpc/clnt.h>
#include <util/perf/perf.h>

namespace sigmaos {
namespace rpc {

bool Clnt::_l = sigmaos::util::log::init_logger(RPCCLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(RPCCLNT_ERR);

std::expected<int, sigmaos::serr::Error> Clnt::BatchFetchDelegatedRPCs(
    std::vector<uint64_t> &rpc_idxs, int n_iov) {
  log(RPCCLNT, "BatchFetchDelegatedRPCs {}", (int)rpc_idxs.size());
  auto out_iov = new sigmaos::io::iovec::IOVec();
  out_iov->AddBuffers(n_iov);
  SigmaMultiDelegatedRPCReq req;
  for (uint64_t rpc_idx : rpc_idxs) {
    req.add_rpcidxs(rpc_idx);
  }
  SigmaMultiDelegatedRPCRep rep;
  Blob blob;
  auto iov = blob.mutable_iov();
  for (int i = 0; i < n_iov; i++) {
    iov->AddAllocated(out_iov->GetBuffer(i)->Get());
  }
  rep.set_allocated_blob(&blob);
  {
    // If there was no cached reply, run the delegated RPC
    auto res = rpc(true, "SPProxySrvAPI.GetMultiDelegatedRPCReplies", req, rep);
    if (!res.has_value()) {
      return res;
    }
  }
  int start = 0;
  for (int i = 0; i < rpc_idxs.size(); i++) {
    int end = start + rep.niovs(i);
    uint64_t rpc_idx = rpc_idxs[i];
    _cache.Register(rpc_idx);
    auto delegated_rep = std::make_shared<SigmaDelegatedRPCRep>();
    auto blob = new Blob();
    auto iov = blob->mutable_iov();
    for (int j = start; j < end; j++) {
      iov->AddAllocated(out_iov->GetBuffer(j)->Get());
    }
    delegated_rep->set_allocated_blob(blob);
    *delegated_rep->mutable_err() = rep.errs(i);
    // Cache the result
    _cache.Put(rpc_idx, delegated_rep, nullptr);
    start = end;
  }
  return 0;
}

// Retrieve the result of a delegated RPC
std::expected<int, sigmaos::serr::Error> Clnt::DelegatedRPC(
    uint64_t rpc_idx, google::protobuf::Message &delegated_rep,
    std::shared_ptr<std::vector<std::shared_ptr<std::string_view>>> views) {
  log(RPCCLNT, "DelegatedRPC {}", (int)rpc_idx);
  // Sanity check
  if ((_shmem && !views) || (!_shmem && views)) {
    fatal("Try delegated RPC with mismatching shmem & views");
  }
  auto out_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
  if (!_shmem) {
    // Prepend empty slots to the out iovec for the marshaled delegated reply
    // and its RPC wrapper
    out_iov->AddBuffers(2);
  }
  // Extract any output IOVecs from the delegated reply RPC
  extract_blob_iov(delegated_rep, out_iov);
  // Create the delegated request
  SigmaDelegatedRPCReq req;
  req.set_rpcidx(rpc_idx);
  req.set_useshmem(_shmem != nullptr);
  auto rep = std::make_shared<SigmaDelegatedRPCRep>();
  Blob blob;
  // Set IOVec buffers if not using shared memory
  auto iov = blob.mutable_iov();
  if (!_shmem) {
    // Add the output buffers for the RPC wrapper & serialized RPC reply, as
    // well as the delegated reply's blob output buffers to the RPC's blob
    for (int i = 0; i < out_iov->Size(); i++) {
      iov->AddAllocated(out_iov->GetBuffer(i)->Get());
    }
  }
  rep->set_allocated_blob(&blob);
  bool reply_cached = false;
  // TODO: remove TS
  auto start = GetCurrentTime();
  {
    auto res = _cache.Get(rpc_idx, out_iov);
    if (!res.has_value()) {
      return res;
    }
    reply_cached = res.value();
  }
  if (reply_cached) {
    log(RPCCLNT, "DelegatedRPC({}) reply cached", (int)rpc_idx);
  } else {
    log(RPCCLNT, "DelegatedRPC({}) reply not cached", (int)rpc_idx);
    // If there was no cached reply, run the delegated RPC
    auto res = rpc(true, "SPProxySrvAPI.GetDelegatedRPCReply", req, *rep);
    if (!res.has_value()) {
      return res;
    }
  }
  // If using shared memory
  if (_shmem) {
    if (rep->useshmem()) {
      // Sanity check that enough buffers were supplied
      if (views->size() != (rep->shmoffs().size() - 2)) {
        fatal("Wrong num buffers supplied for shared-memory delRPC: {} != {}",
              views->size(), rep->shmoffs().size());
      }
      log(RPCCLNT, "DelegatedRPC({}) using shared memory", (int)rpc_idx);
      // Set the output buffers for the RPC wrapper & serialized RPC reply
      for (int i = 0; i < 2; i++) {
        uint64_t off = rep->shmoffs(i);
        size_t len = (size_t)rep->shmlens(i);
        auto b =
            std::make_shared<std::string>((char *)_shmem->GetBuf() + off, len);
        out_iov->AppendBuffer(std::make_shared<sigmaos::io::iovec::Buffer>(b));
      }
      // Skip the first two IOVs since those are the RPC wrapper & RPC struct
      for (int i = 2; i < rep->shmoffs().size(); i++) {
        uint64_t off = rep->shmoffs(i);
        size_t len = (size_t)rep->shmlens(i);
        views->at(i - 2) = std::make_shared<std::string_view>(
            (char *)_shmem->GetBuf() + off, len);
      }
      log(RPCCLNT, "DelegatedRPC({}) using shared memory set views successful",
          (int)rpc_idx);
    }
  }
  // Process the delegated, wrapped RPC reply
  start = GetCurrentTime();
  auto res = process_wrapped_reply(rpc_idx, out_iov, delegated_rep);
  log(ALWAYS, "DelegatedRPC({}) process wrapped reply latency {}ms",
      (int)rpc_idx, LatencyMS(start));
  log(RPCCLNT, "DelegatedRPC done {}", (int)rpc_idx);
  {
    // Release ownership of blob (which is a local stack variable declared
    // above)
    auto blob = rep->release_blob();
  }
  return res;
}

// Perform an RPC
std::expected<int, sigmaos::serr::Error> Clnt::RPC(
    std::string method, google::protobuf::Message &req,
    google::protobuf::Message &rep) {
  {
    auto res = check_channel_init();
    if (!res.has_value()) {
      return res;
    }
  }
  return rpc(false, method, req, rep);
}

std::expected<int, sigmaos::serr::Error> Clnt::check_channel_init() {
  // Fast-path: check if channel is initialized
  if (_chan->IsInitialized()) {
    return 0;
  }

  std::lock_guard<std::mutex> guard(_mu);
  // Slow-path: check again, now holding the lock
  if (_chan->IsInitialized()) {
    return 0;
  }
  // Initialize the channel
  auto res = _chan->Init();
  if (!res.has_value()) {
    log(RPCCLNT_ERR, "Error initialize channel: {}", res.error().String());
    return res;
  }
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::rpc(
    bool delegate, std::string method, google::protobuf::Message &req,
    google::protobuf::Message &rep) {
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
  uint64_t seqno = _seqno++;
  // Wrap the RPC and execute it
  auto res = wrap_and_run_rpc(delegate, seqno, method, in_iov, out_iov);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  // Process the wrapped response
  return process_wrapped_reply(seqno, out_iov, rep);
}

std::expected<int, sigmaos::serr::Error> Clnt::wrap_and_run_rpc(
    bool delegate, uint64_t seqno, std::string method,
    const std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov,
    std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov) {
  auto wrapped_in_iov = std::make_shared<sigmaos::io::iovec::IOVec>();
  Req req;
  auto wrapper_req_data = std::make_shared<std::string>();
  req.set_method(method);
  // Wrap the request, serialize the wrapper, and write it together with the
  // request data.
  req.SerializeToString(wrapper_req_data.get());
  wrapped_in_iov->AppendBuffer(
      std::make_shared<sigmaos::io::iovec::Buffer>(wrapper_req_data));
  wrapped_in_iov->Resize(in_iov->Size() + 1);
  for (int i = 0; i < in_iov->Size(); i++) {
    wrapped_in_iov->SetBuffer(i + 1, in_iov->GetBuffer(i));
  }

  // Create the call object to be sent, and perform the RPC.
  auto wrapped_call =
      std::make_shared<io::transport::Call>(seqno, wrapped_in_iov, out_iov);
  auto start = GetCurrentTime();
  std::expected<std::shared_ptr<sigmaos::io::transport::Call>,
                sigmaos::serr::Error>
      res;
  if (delegate) {
    // Sanity check
    if (!_delegate_chan) {
      fatal("Try to run a delegated RPC with a null delegate channel");
    }
    res = _delegate_chan->SendReceive(wrapped_call);
  } else {
    res = _chan->SendReceive(wrapped_call);
  }
  if (!res.has_value()) {
    log(RPCCLNT_ERR, "Error sendreceive: {}", res.error().String());
    return std::unexpected(res.error());
  }
  log(PROXY_RPC_LAT, "RPCClnt SendReceive");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::process_wrapped_reply(
    uint64_t seqno, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov,
    google::protobuf::Message &rep) {
  log(RPCCLNT, "Deserialize wrapper for reply seqno: {}", seqno);
  Rep wrapped_rep;
  // Deserialize the wrapper
  auto wrapper_rep_buf = out_iov->GetBuffer(0);
  auto start = GetCurrentTime();
  wrapped_rep.ParseFromString(*wrapper_rep_buf->Get());
  log(RPCCLNT, "Remove wrapper buffer for reply seqno: {}", seqno);
  log(PROXY_RPC_LAT, "RPCClnt Parse wrapper");
  // Remove the wrapper from the out IOVec
  out_iov->RemoveBuffer(0);
  log(RPCCLNT, "Done remove wrapper buffer for reply seqno: {}", seqno);
  // TODO: Record stats
  //	rpcc.si.Stat(method, time.Since(start).Microseconds())
  // If there was an error in the RPC stack, bail out
  if (wrapped_rep.err().errcode() != sigmaos::serr::TErrNoError) {
    log(RPCCLNT_ERR, "error in RPC stack");
    log(RPCCLNT_ERR, "error in RPC stack error:{}",
        wrapped_rep.err().ShortDebugString());
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::TErrUnreachable,
        std::format("rpc error: {}", wrapped_rep.err().ShortDebugString())));
  }
  log(RPCCLNT, "Deserialize reply data");
  // Deserialize the reply
  auto rep_data_buf = out_iov->GetBuffer(0);
  start = GetCurrentTime();
  rep.ParseFromString(*rep_data_buf->Get());
  log(PROXY_RPC_LAT, "RPCClnt Parse reply lat:{}ms", (int)LatencyMS(start));
  log(RPCCLNT, "Remove reply data buffer");
  // Remove the first element in iov, which contains the serialized reply
  // message
  out_iov->RemoveBuffer(0);
  log(RPCCLNT, "Set blob IOV len {}", out_iov->Size());
  // Set the reply's blob's IOV to point to the returned data, if applicable.
  set_blob_iov(out_iov, rep);
  log(RPCCLNT, "Done set blob IOV");
  return 0;
}

};  // namespace rpc
};  // namespace sigmaos
