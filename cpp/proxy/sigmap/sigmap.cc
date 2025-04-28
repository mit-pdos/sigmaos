#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

namespace sigmaos {
namespace proxy::sigmap {

std::expected<int, std::string> Clnt::Test() {
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
