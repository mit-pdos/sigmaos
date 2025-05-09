#include <io/conn/conn.h>

#include <unistd.h>
#include <format>

#include <util/codec/codec.h>

namespace sigmaos {
namespace io::conn {

bool UnixConn::_l = sigmaos::util::log::init_logger(CONN);
bool UnixConn::_l_e = sigmaos::util::log::init_logger(CONN_ERR);

std::expected<int, sigmaos::serr::Error> UnixConn::Read(std::string *b) {
  return read_bytes(b->data(), b->size());
}

std::expected<int, sigmaos::serr::Error> UnixConn::Write(const std::string *b) {
  return write_bytes(b->data(), b->size());
}

std::expected<uint64_t, sigmaos::serr::Error> UnixConn::ReadUint64() {
  size_t size = sizeof(uint64_t);
  char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint64(b);
}

std::expected<int, sigmaos::serr::Error> UnixConn::WriteUint64(uint64_t i) {
  size_t size = sizeof(uint64_t);
  char b[size];
  sigmaos::util::codec::uint64_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<uint32_t, sigmaos::serr::Error> UnixConn::ReadUint32() {
  size_t size = sizeof(uint32_t);
  char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint32(b);
}

std::expected<int, sigmaos::serr::Error> UnixConn::WriteUint32(uint32_t i) {
  size_t size = sizeof(uint32_t);
  char b[size];
  sigmaos::util::codec::uint32_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<int, sigmaos::serr::Error> UnixConn::Close() {
  log(CONN, "Closing unix conn");
  // Close the socket FD
  // TODO: have the reader actually close the FD, or else it may block
  // indefinitely, since closing while reading is UB.
  int err = close(_sockfd);
  if (err) {
    throw std::runtime_error(std::format("Error close sockfd: {}", err));
  }
  log(CONN, "Done closing unix conn");
  return 0;
}

std::expected<int, sigmaos::serr::Error> UnixConn::read_bytes(char *b, size_t size) {
  int n = read(_sockfd, b, size);
  if (n != size) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, std::format("read wrong num bytes: {} != {}", n, size)));
  }
  return n;
}

std::expected<int, sigmaos::serr::Error> UnixConn::write_bytes(const char *b, size_t size) {
  int n = write(_sockfd, b, size);
  if (n != size) {
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, std::format("wrote wrong num bytes: {} != {}", n, size)));
  }
  return n;
}

};
};
