#pragma once

#include <expected>
#include <atomic>

#include <google/protobuf/message.h>

#include <util/log/log.h>
#include <io/demux/demux.h>
#include <rpc/proto/rpc.pb.h>

namespace sigmaos {
namespace rpc {

const std::string RPCCLNT = "RPCCLNT";
const std::string RPCCLNT_ERR = "RPCCLNT" + sigmaos::util::log::ERR;

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::io::demux::Clnt> demux) : _seqno(1), _demux(demux) {}
  ~Clnt() { Close(); }

  std::expected<int, std::string> RPC(std::string method, const google::protobuf::Message &req, google::protobuf::Message &res);
  void Close() { 
    log(RPCCLNT, "Close");
    _demux->Close(); 
    log(RPCCLNT, "Done close");
  }
  private:
  std::atomic<uint64_t> _seqno;
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<Rep, std::string> wrap_and_run_rpc(std::string method, const std::vector<std::vector<unsigned char>> &in_iov, std::vector<std::vector<unsigned char>> &out_iov);
};

};
};
