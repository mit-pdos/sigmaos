#include <util/codec/codec.h>

namespace sigmaos {
namespace util::codec {

uint32_t bytes_to_uint32(char *b) {
  return (((uint32_t)b[0]) & 0xFF) | (((uint32_t)b[1]) & 0xFF) << 8 |
         (((uint32_t)b[2]) & 0xFF) << 16 | (((uint32_t)b[3]) & 0xFF) << 24;
}

void uint32_to_bytes(char *b, uint32_t i) {
  b[0] = (char)(i);
  b[1] = (char)(i >> 8);
  b[2] = (char)(i >> 16);
  b[3] = (char)(i >> 24);
}

uint64_t bytes_to_uint64(char *b) {
  return (((uint64_t)b[0]) & 0xFF) | (((uint64_t)b[1]) & 0xFF) << 8 |
         (((uint64_t)b[2]) & 0xFF) << 16 | (((uint64_t)b[3]) & 0xFF) << 24 |
         (((uint64_t)b[4]) & 0xFF) << 32 | (((uint64_t)b[5]) & 0xFF) << 40 |
         (((uint64_t)b[6]) & 0xFF) << 48 | (((uint64_t)b[7]) & 0xFF) << 56;
}

void uint64_to_bytes(char *b, uint64_t i) {
  b[0] = (char)(i);
  b[1] = (char)(i >> 8);
  b[2] = (char)(i >> 16);
  b[3] = (char)(i >> 24);
  b[4] = (char)(i >> 32);
  b[5] = (char)(i >> 40);
  b[6] = (char)(i >> 48);
  b[7] = (char)(i >> 56);
}

};  // namespace util::codec
};  // namespace sigmaos
