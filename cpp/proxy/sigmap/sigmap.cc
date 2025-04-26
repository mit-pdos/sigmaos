#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>

namespace sigmaos {
namespace proxy::sigmap {

std::expected<int, std::string> Clnt::Test() {
  std::string msg = "hello!";
  auto b = std::vector<unsigned char>((unsigned char *) msg.data(), (unsigned char *) msg.data() + msg.length());
  sigmaos::io::frame::WriteFrame(_conn, b);
  std::vector<unsigned char> b2;
  b2.resize(100);
  auto res = sigmaos::io::frame::ReadFrameIntoVec(_conn, b2);
  if (!res.has_value()) {
    std::cout << "err read frame " << res.error() << std::endl;
  }
  std::string msg2(b2.begin(), b2.end());
  std::cout << "response: " << msg2 << std::endl;
  return 0;
}

};
};
