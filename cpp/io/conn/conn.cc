#include <io/conn/conn.h>

#include <util/codec/codec.h>

namespace sigmaos {
namespace io::conn {

const std::string CONN = "CONN";
const std::string CONN_ERR = CONN + sigmaos::util::log::ERR;

std::expected<int, sigmaos::serr::Error> Conn::Read(std::string *b) {
  return read_bytes(b->data(), b->size());
}

std::expected<int, sigmaos::serr::Error> Conn::Write(const std::string *b) {
  return write_bytes(b->data(), b->size());
}

std::expected<uint64_t, sigmaos::serr::Error> Conn::ReadUint64() {
  size_t size = sizeof(uint64_t);
  char b[size] = {0};
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint64(b);
}

std::expected<int, sigmaos::serr::Error> Conn::WriteUint64(uint64_t i) {
  size_t size = sizeof(uint64_t);
  char b[size] = {0};
  sigmaos::util::codec::uint64_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<uint32_t, sigmaos::serr::Error> Conn::ReadUint32() {
  size_t size = sizeof(uint32_t);
  char b[size] = {0};
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint32(b);
}

std::expected<int, sigmaos::serr::Error> Conn::WriteUint32(uint32_t i) {
  size_t size = sizeof(uint32_t);
  char b[size] = {0};
  sigmaos::util::codec::uint32_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<int, sigmaos::serr::Error> Conn::Close() {
  log(CONN, "Closing unix conn");
  // Close the socket FD
  // TODO: have the reader actually close the FD, or else it may block
  // indefinitely, since closing while reading is UB.
  int err = close(_sockfd);
  if (err) {
    fatal("Error close sockfd: {}", err);
  }
  log(CONN, "Done closing unix conn");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Conn::read_bytes(char *b, size_t size) {
  int total = 0;
  while (total != size) {
    int n = read(_sockfd, b, size);
    // EOF
    if (n == 0) {
      break;
    }
    // Error
    if (n == -1) {
      log(CONN_ERR, "Err read_bytes fd {}", _sockfd);
      return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, "read error"));
    }
    // Success
    // Move the pointer into the buffer forward
    b += n;
    // Increment the total number of bytes read
    total += n;
  }
  if (total != size) {
    log(CONN_ERR, "Err read_bytes fd {} n({:d}) != size({:d})", _sockfd, (int) total, (int) size);
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, std::format("read wrong num bytes: {} != {}", (int) total, (int) size)));
  }
  return total;
}

// TODO: multi-step writes
std::expected<int, sigmaos::serr::Error> Conn::write_bytes(const char *b, size_t size) {
  int total = 0;
  while (total != size) {
    int n = write(_sockfd, b, size);
    // EOF
    if (n == 0) {
      break;
    }
    // Error
    if (n == -1) {
      log(CONN_ERR, "Err write_bytes fd {}", _sockfd);
      return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, "write error"));
    }
    // Success
    // Move the pointer into the buffer forward
    b += n;
    // Increment the total number of bytes read
    total += n;
  }
  if (total != size) {
    log(CONN_ERR, "Err write_bytes fd {}", _sockfd);
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::Terror::TErrUnreachable, std::format("wrote wrong num bytes: {} != {}", (int) total, (int) size)));
  }
  return total;
}

};
};
