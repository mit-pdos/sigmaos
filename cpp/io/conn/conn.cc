#include <io/conn/conn.h>

#include <unistd.h>
#include <format>

#include <util/codec/codec.h>

namespace sigmaos {
namespace io::conn {

std::expected<int, std::string> UnixConn::Read(std::vector<unsigned char> &b) {
  return read_bytes(b.data(), b.size());
}

std::expected<int, std::string> UnixConn::Write(const std::vector<unsigned char> &b) {
  return write_bytes(b.data(), b.size());
}

std::expected<uint64_t, std::string> UnixConn::ReadUint64() {
  size_t size = sizeof(uint64_t);
  unsigned char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint64(b);
}

std::expected<int, std::string> UnixConn::WriteUint64(uint64_t i) {
  size_t size = sizeof(uint64_t);
  unsigned char b[size];
  sigmaos::util::codec::uint64_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<uint32_t, std::string> UnixConn::ReadUint32() {
  size_t size = sizeof(uint32_t);
  unsigned char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return sigmaos::util::codec::bytes_to_uint32(b);
}

std::expected<int, std::string> UnixConn::WriteUint32(uint32_t i) {
  size_t size = sizeof(uint32_t);
  unsigned char b[size];
  sigmaos::util::codec::uint32_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return res;
  }
  return size;
}

std::expected<int, std::string> UnixConn::Close() {
  std::cout << "Closing conn" << std::endl;
  int err = close(_sockfd);
  if (err) {
    throw std::runtime_error(std::format("Error close sockfd: {}", err));
  }
  std::cout << "Done closing conn" << std::endl;
  return 0;
}

std::expected<int, std::string> UnixConn::read_bytes(unsigned char *b, size_t size) {
  int n = read(_sockfd, b, size);
  if (n != size) {
    return std::unexpected(std::format("read wrong num bytes: {} != {}", n, size));
  }
  return n;
}

std::expected<int, std::string> UnixConn::write_bytes(const unsigned char *b, size_t size) {
  int n = write(_sockfd, b, size);
  if (n != size) {
    return std::unexpected(std::format("wrote wrong num bytes: {} != {}", n, size));
  }
  return n;
}

};
};
