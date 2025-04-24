#include <proxy/sigmap/sigmap.h>

#include <io/frame/frame.h>

namespace sigmaos {
namespace proxy::sigmap {

std::expected<int, std::string> Clnt::Test() {
  std::string msg = "hello!";
  auto b = std::vector<unsigned char>((unsigned char *) msg.data(), (unsigned char *) msg.data() + msg.length());
  sigmaos::io::frame::WriteFrame(conn, b);
  return 0;
}

};
};
