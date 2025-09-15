#include "rpc/delegation/cache.h"

namespace sigmaos {
namespace rpc {
namespace delegation {

bool Cache::_l = sigmaos::util::log::init_logger(RPCCLNT_CACHE);
bool Cache::_l_e = sigmaos::util::log::init_logger(RPCCLNT_CACHE_ERR);

void Cache::Register(uint64_t rpc_idx) {
  log(RPCCLNT_CACHE, "Register cachable DelegatedRPC RPC({})", (int)rpc_idx);
  std::lock_guard<std::mutex> lock(_mu);
  // Sanity check
  if (_registered.contains(rpc_idx)) {
    fatal("Double-register RPC idx {}", (int)rpc_idx);
  }
  _registered[rpc_idx] = true;
  _done[rpc_idx] = false;
}

void Cache::Put(uint64_t rpc_idx, std::shared_ptr<SigmaDelegatedRPCRep> rep,
                std::shared_ptr<sigmaos::serr::Error> err) {
  {
    log(RPCCLNT_CACHE, "Put cached DelegatedRPC RPC({}) err {}", (int)rpc_idx,
        (err != nullptr));
    std::lock_guard<std::mutex> lock(_mu);
    // Sanity check
    if (!_registered.contains(rpc_idx)) {
      fatal("Complete unregistered RPC({})", (int)rpc_idx);
    }
    // Sanity check
    if (_done.contains(rpc_idx) && _done.at(rpc_idx)) {
      fatal("Complete already-completed RPC({})", (int)rpc_idx);
    }
    // Store result
    _reps[rpc_idx] = rep;
    _done[rpc_idx] = true;
    _errors[rpc_idx] = err;
  }
  // After releasing the lock, broadcast to waiters that a reply has been
  // received
  _cond.notify_all();
}

std::expected<bool, sigmaos::serr::Error> Cache::Get(
    uint64_t rpc_idx, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov) {
  log(RPCCLNT_CACHE, "Get cached DelegatedRPC RPC({})", (int)rpc_idx);
  // Acquire the lock
  std::unique_lock<std::mutex> lock(_mu);

  // Delegated RPC retrieval is not in-progress, so bail out
  if (!_registered.contains(rpc_idx)) {
    log(RPCCLNT_CACHE, "No cached DelegatedRPC registered RPC({})",
        (int)rpc_idx);
    return false;
  }

  // Wait for the delegated RPC retrieval to complete
  _cond.wait(lock, [this, rpc_idx] { return _done.at(rpc_idx); });

  log(RPCCLNT_CACHE, "Done waiting for cached DelegatedRPC RPC({})",
      (int)rpc_idx);

  // Sanity check that the RPC indeed completed
  if (!_done.at(rpc_idx)) {
    fatal("Stopped waiting despite lack of RPC({}) completion", (int)rpc_idx);
  }
  // Copy reply to output
  auto cached_reply = _reps.at(rpc_idx);
  // Set the wrapper & serialized RPC buffers since these two are prepended
  // by the RPC stack and aren't supplied by the caller
  for (int i = 0; i < 2; i++) {
    out_iov->SetBuffer(i, std::make_shared<sigmaos::io::iovec::Buffer>(
                              cached_reply->mutable_blob()->mutable_iov(i)));
  }
  // Copy into the remaining IOVs in the blob, since these are supplied by the
  // caller
  for (int i = 2; i < cached_reply->blob().iov().size(); i++) {
    *(out_iov->GetBuffer(i)->Get()) = cached_reply->blob().iov(i);
  }
  //  *rep->mutable_err() = cached_reply->err();

  auto err = _errors.at(rpc_idx);
  if (err) {
    return std::unexpected(*_errors[rpc_idx]);
  }
  return true;
}

};  // namespace delegation
};  // namespace rpc
};  // namespace sigmaos
