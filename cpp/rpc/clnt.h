#pragma once

#include <google/protobuf/message.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <proxy/sigmap/proto/spproxy.pb.h>
#include <rpc/delegation/cache.h>
#include <rpc/proto/rpc.pb.h>
#include <serr/serr.h>
#include <util/log/log.h>

#include <atomic>
#include <expected>

namespace sigmaos {
namespace rpc {

const std::string RPCCLNT = "RPCCLNT";
const std::string RPCCLNT_ERR = "RPCCLNT" + sigmaos::util::log::ERR;

class Clnt {
 public:
  Clnt(std::shared_ptr<Channel> chan)
      : _seqno(1), _chan(chan), _delegate_chan(nullptr), _cache() {}
  Clnt(std::shared_ptr<Channel> chan, std::shared_ptr<Channel> delegate_chan)
      : _seqno(1), _chan(chan), _delegate_chan(delegate_chan), _cache() {}
  ~Clnt() { Close(); }

  std::shared_ptr<Channel> GetChannel() { return _chan; }
  std::expected<int, sigmaos::serr::Error> RPC(std::string method,
                                               google::protobuf::Message &req,
                                               google::protobuf::Message &rep);
  std::expected<int, sigmaos::serr::Error> DelegatedRPC(
      uint64_t rpc_idx, google::protobuf::Message &delegated_rep);
  std::expected<int, sigmaos::serr::Error> BatchFetchDelegatedRPCs(
      std::vector<uint64_t> &rpc_idxs, int n_iov);
  void Close() {
    log(RPCCLNT, "Close");
    _chan->Close();
    log(RPCCLNT, "Done close");
  }

 private:
  std::mutex _mu;
  std::atomic<uint64_t> _seqno;
  std::shared_ptr<Channel> _chan;
  std::shared_ptr<Channel> _delegate_chan;
  sigmaos::rpc::delegation::Cache _cache;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<int, sigmaos::serr::Error> check_channel_init();
  std::expected<int, sigmaos::serr::Error> rpc(bool delegate,
                                               std::string method,
                                               google::protobuf::Message &req,
                                               google::protobuf::Message &rep);
  std::expected<int, sigmaos::serr::Error> wrap_and_run_rpc(
      bool delegate, uint64_t seqno, std::string method,
      const std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov,
      std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov);
  std::expected<int, sigmaos::serr::Error> process_wrapped_reply(
      uint64_t seqno, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov,
      google::protobuf::Message &rep);
};

};  // namespace rpc
};  // namespace sigmaos
