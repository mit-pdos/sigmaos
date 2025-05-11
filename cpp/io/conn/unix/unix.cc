#include <io/conn/unix/unix.h>

#include <unistd.h>
#include <format>

namespace sigmaos {
namespace io::conn::unixconn {

bool Conn::_l = sigmaos::util::log::init_logger(UNIXCONN);
bool Conn::_l_e = sigmaos::util::log::init_logger(UNIXCONN_ERR);

};
};
