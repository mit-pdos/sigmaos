#include <io/conn/conn.h>

#include <unistd.h>
#include <format>

namespace sigmaos {
namespace io::conn {

// Decoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=90;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
uint32_t bytes_to_uint32(unsigned char *b) {
  return uint32_t(b[0]) | uint32_t(b[1]) << 8 | uint32_t(b[2]) << 16 | 
    uint32_t(b[3]) << 24;
}

// Encoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=96;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
void uint32_to_bytes(unsigned char *b, uint32_t i) {
	b[0] = (unsigned char) (i);
	b[1] = (unsigned char) (i >> 8);
	b[2] = (unsigned char) (i >> 16);
	b[3] = (unsigned char) (i >> 24);
}

// Decoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=115;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
uint64_t bytes_to_uint64(unsigned char *b) {
  return uint64_t(b[0]) | uint64_t(b[1]) << 8 | uint64_t(b[2]) << 16 |
    uint64_t(b[3]) << 24 | uint64_t(b[4]) << 32 | uint64_t(b[5]) << 40 |
    uint64_t(b[6]) << 48 | uint64_t(b[7]) << 56;
}

// Encoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=122;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a;bpv=0;bpt=1
void uint64_to_bytes(unsigned char *b, uint64_t i) {
	b[0] = (unsigned char) (i);
	b[1] = (unsigned char) (i >> 8);
	b[2] = (unsigned char) (i >> 16);
	b[3] = (unsigned char) (i >> 24);
	b[4] = (unsigned char) (i >> 32);
	b[5] = (unsigned char) (i >> 40);
	b[6] = (unsigned char) (i >> 48);
	b[7] = (unsigned char) (i >> 56);
}

std::expected<int, std::string> UnixConn::Read(std::vector<unsigned char> b) {
  return read_bytes(b.data(), b.size());
}

std::expected<int, std::string> UnixConn::Write(std::vector<unsigned char> b) {
  return write_bytes(b.data(), b.size());
}

std::expected<uint64_t, std::string> UnixConn::ReadUint64() {
  size_t size = sizeof(uint64_t);
  unsigned char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return bytes_to_uint64(b);
}

std::expected<int, std::string> UnixConn::WriteUint64(uint64_t i) {
  size_t size = sizeof(uint64_t);
  unsigned char b[size];
  uint64_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return size;
}

std::expected<uint32_t, std::string> UnixConn::ReadUint32() {
  size_t size = sizeof(uint32_t);
  unsigned char b[size];
  auto res = read_bytes(b, size);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return bytes_to_uint32(b);
}

std::expected<int, std::string> UnixConn::WriteUint32(uint32_t i) {
  size_t size = sizeof(uint32_t);
  unsigned char b[size];
  uint32_to_bytes(b, i);
  auto res = write_bytes(b, size);
  if (!res.has_value()) {
    return std::unexpected(res.error());
  }
  return size;
}


std::expected<int, std::string> UnixConn::read_bytes(unsigned char *b, size_t size) {
  int n = read(sockfd, b, size);
  if (n != size) {
    return std::unexpected(std::format("read wrong num bytes: {} != {}", n, size));
  }
  return n;
}

std::expected<int, std::string> UnixConn::write_bytes(unsigned char *b, size_t size) {
  int n = write(sockfd, b, size);
  if (n != size) {
    return std::unexpected(std::format("wrote wrong num bytes: {} != {}", n, size));
  }
  return n;
}

};
};
