#pragma once

#include <expected>

#include <google/protobuf/message.h>

#include <io/demux/demux.h>
#include <rpc/proto/rpc.pb.h>

namespace sigmaos {
namespace rpc {

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::io::demux::Clnt> demux) : _seqno(0), _demux(demux) {}
  ~Clnt() { throw std::runtime_error("unimplemented"); }

  std::expected<int, std::string> RPC(std::string method, const google::protobuf::Message &req, google::protobuf::Message &res);
  private:
  uint64_t _seqno;
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;

  std::expected<Rep, std::string> wrap_and_run_rpc(std::string method, const std::vector<std::vector<unsigned char>> &in_iov, std::vector<std::vector<unsigned char>> &out_iov);
};

};
};
