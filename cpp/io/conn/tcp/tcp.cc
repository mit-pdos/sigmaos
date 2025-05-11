#include <io/conn/tcp/tcp.h>

namespace sigmaos {
namespace io::conn::tcpconn {

bool Conn::_l = sigmaos::util::log::init_logger(TCPCONN);
bool Conn::_l_e = sigmaos::util::log::init_logger(TCPCONN_ERR);

};
};
