#pragma once

#include <expected>

#include <google/protobuf/message.h>

#include <io/demux/demux.h>

namespace sigmaos {
namespace rpc {

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::io::demux::Clnt> demux) : demux(demux) {}
  ~Clnt() { throw std::runtime_error("unimplemented"); }

  std::expected<int, std::string> RPC(std::string method, const google::protobuf::Message &req, google::protobuf::Message &res);
  private:
  std::shared_ptr<sigmaos::io::demux::Clnt> _demux;
};

};
};
