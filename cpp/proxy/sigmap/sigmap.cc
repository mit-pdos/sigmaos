#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

namespace sigmaos {
namespace proxy::sigmap {

std::expected<int, std::string> Clnt::Test() {
  // Initialize the sigmaproxyd connection
  {
    SigmaInitReq req;
    SigmaErrRep rep;
    // Set the proc env proto
    req.set_allocated_procenvproto(_env->GetProto());
    // Execute the RPC
    auto res = _rpcc->RPC("SPProxySrvAPI.Init", req, rep);
    if (!res.has_value()) {
      std::cout << "Err RPC: " << res.error() << std::endl;
      return res;
    }
    if (rep.err().errcode() != 0) {
      throw std::runtime_error(std::format("init rpc error: {}", rep.err().DebugString()));
    }
    std::cout << "Init RPC successful!" << std::endl;
    // Make sure to release the proc env proto pointer so it isn't destroyed
    req.release_procenvproto();
  }
  {
    std::cout << "Init RPC successful!" << std::endl;
    SigmaNullReq req;
    SigmaClntIdRep rep;
    auto res = _rpcc->RPC("SPProxySrvAPI.ClntId", req, rep);
    if (!res.has_value()) {
      std::cout << "Err RPC: " << res.error() << std::endl;
      return res;
    }
    std::cout << "ClntID RPC successful! rep " << rep.clntid() << std::endl;
  }
  return 0;
}

};
};
