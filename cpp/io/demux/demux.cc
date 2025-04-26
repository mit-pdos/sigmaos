#include <io/demux/demux.h>

namespace sigmaos {
namespace io::demux {

std::expected<bool, std::string> SendReceive(const void *call, std::vector<std::vector<unsigned char>> outiov) {
  throw std::runtime_error("unimplmented");
}

std::expected<bool, std::string> Close() {
  throw std::runtime_error("unimplmented");
}

bool IsClosed() {
  throw std::runtime_error("unimplmented");
}

};
};
