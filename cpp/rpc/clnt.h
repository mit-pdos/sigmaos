#pragma once

#include <expected>
#include <atomic>

#include <google/protobuf/message.h>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <rpc/proto/rpc.pb.h>

namespace sigmaos {
namespace rpc {

const std::string RPCCLNT = "RPCCLNT";
const std::string RPCCLNT_ERR = "RPCCLNT" + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt(std::shared_ptr<Channel> chan) : _seqno(1), _chan(chan) {}
  ~Clnt() { Close(); }

  std::expected<int, sigmaos::serr::Error> RPC(std::string method, google::protobuf::Message &req, google::protobuf::Message &res);
  void Close() { 
    log(RPCCLNT, "Close");
    _chan->Close(); 
    log(RPCCLNT, "Done close");
  }
  private:
  std::atomic<uint64_t> _seqno;
  std::shared_ptr<Channel> _chan;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<Rep, sigmaos::serr::Error> wrap_and_run_rpc(std::string method, const std::shared_ptr<sigmaos::io::iovec::IOVec> in_iov, std::shared_ptr<sigmaos::io::iovec::IOVec> out_iov);
};

};
};
