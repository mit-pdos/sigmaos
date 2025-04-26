#include <rpc/rpc.h>

namespace sigmaos {
namespace rpc {

std::expected<int, std::string> Clnt::RPC(std::string method, const google::protobuf::Message &req, google::protobuf::Message &res) {
  throw std::runtime_error("unimplemented");
}

};
};
