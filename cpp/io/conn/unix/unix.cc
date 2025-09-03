#include <io/conn/unix/unix.h>

namespace sigmaos {
namespace io::conn::unixconn {

bool Conn::_l = sigmaos::util::log::init_logger(UNIXCONN);
bool Conn::_l_e = sigmaos::util::log::init_logger(UNIXCONN_ERR);

};  // namespace io::conn::unixconn
};  // namespace sigmaos
