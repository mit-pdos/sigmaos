#include <io/conn/tcp/tcp.h>

namespace sigmaos {
namespace io::conn::tcpconn {

bool Conn::_l = sigmaos::util::log::init_logger(TCPCONN);
bool Conn::_l_e = sigmaos::util::log::init_logger(TCPCONN_ERR);

void set_tcp_nodelay(int sockfd) {
  int flag = 1;
  if (setsockopt(sockfd, IPPROTO_TCP, TCP_NODELAY, (char *)&flag,
                 sizeof(int))) {
    close(sockfd);
    log(TCPCONN_ERR, "Failed to set TCP socket options");
    fatal("Failed to set TCP socket options");
  }
}

std::expected<std::shared_ptr<Conn>, sigmaos::serr::Error> Listener::Accept() {
  struct sockaddr_in clnt_addr = {0};
  socklen_t addr_len = sizeof(_addr);
  int connfd = accept(_sockfd, (struct sockaddr *)&clnt_addr, &addr_len);
  if (connfd == -1) {
    log(TCPCONN_ERR, "Error accept socket fd {}", _sockfd);
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::TErrError, std::format("accept socket fd {}", _sockfd)));
  }
  set_tcp_nodelay(connfd);
  return std::make_shared<Conn>(_id, connfd, clnt_addr);
}

std::expected<int, sigmaos::serr::Error> Listener::Close() {
  if (close(_sockfd)) {
    log(TCPCONN_ERR, "Error close socket fd {}", _sockfd);
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::TErrError, std::format("close socket fd {}", _sockfd)));
  }
  return 0;
}

};  // namespace io::conn::tcpconn
};  // namespace sigmaos
