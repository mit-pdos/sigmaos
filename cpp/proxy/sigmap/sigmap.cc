#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

namespace sigmaos {
namespace proxy::sigmap {

std::expected<int, std::string> Clnt::Test() {
  {
    SigmaInitReq req;
    SigmaErrRep rep;
    req.set_allocated_procenvproto(_env->GetProto());
    auto res = _rpcc->RPC("SPProxySrvAPI.Init", req, rep);
    if (!res.has_value()) {
      std::cout << "Err RPC: " << res.error() << std::endl;
      return res;
    }
//    if (rep.Rerror
    std::cout << "Init RPC successful!" << std::endl;
  }
  {
    SigmaNullReq req;
    SigmaClntIdRep rep;
    auto res = _rpcc->RPC("SPProxySrvAPI.ClntId", req, rep);
    if (!res.has_value()) {
      std::cout << "Err RPC: " << res.error() << std::endl;
      return res;
    }
    std::cout << "ClntID RPC successful! rep " << rep.clntid() << std::endl;
  }
//  std::string msg = "hello!";
//  auto b = std::vector<unsigned char>((unsigned char *) msg.data(), (unsigned char *) msg.data() + msg.length());
//  sigmaos::io::frame::WriteFrame(_conn, b);
//  std::vector<unsigned char> b2;
//  b2.resize(100);
//  auto res = sigmaos::io::frame::ReadFrameIntoVec(_conn, b2);
//  if (!res.has_value()) {
//    std::cout << "err read frame " << res.error() << std::endl;
//  }
//  std::string msg2(b2.begin(), b2.end());
//  std::cout << "response: " << msg2 << std::endl;
  return 0;
}

};
};
