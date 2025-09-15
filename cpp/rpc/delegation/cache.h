#pragma once

#include <io/iovec/iovec.h>
#include <proxy/sigmap/proto/spproxy.pb.h>
#include <serr/serr.h>
#include <util/log/log.h>

#include <expected>
#include <memory>
#include <mutex>
#include <unordered_map>

namespace sigmaos {
namespace rpc {
namespace delegation {

const std::string RPCCLNT_CACHE = "RPCCLNT_CACHE";
const std::string RPCCLNT_CACHE_ERR = "RPCCLNT_CACHE" + sigmaos::util::log::ERR;

class Cache {
 public:
  Cache() : _reps(), _errors(), _done(), _registered() {}
  ~Cache() {}

  void Register(uint64_t rpc_idx);

  // Store a delegated RPC reply with the given index
  void Put(uint64_t rpc_idx, std::shared_ptr<SigmaDelegatedRPCRep> response,
           std::shared_ptr<sigmaos::serr::Error> err);

  // Retrieve a delegated RPC reply by index
  std::expected<bool, sigmaos::serr::Error> Get(
      uint64_t rpc_idx, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov);

 private:
  mutable std::mutex _mu;
  std::condition_variable _cond;
  std::map<uint64_t, std::shared_ptr<SigmaDelegatedRPCRep>> _reps;
  std::map<uint64_t, std::shared_ptr<sigmaos::serr::Error>> _errors;
  std::map<uint64_t, bool> _done;
  std::map<uint64_t, bool> _registered;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};  // namespace delegation
};  // namespace rpc
};  // namespace sigmaos
