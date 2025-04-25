#include <util/codec/codec.h>

namespace sigmaos {
namespace util::codec {

uint32_t bytes_to_uint32(unsigned char *b) {
  return uint32_t(b[0]) | uint32_t(b[1]) << 8 | uint32_t(b[2]) << 16 | 
    uint32_t(b[3]) << 24;
}

void uint32_to_bytes(unsigned char *b, uint32_t i) {
	b[0] = (unsigned char) (i);
	b[1] = (unsigned char) (i >> 8);
	b[2] = (unsigned char) (i >> 16);
	b[3] = (unsigned char) (i >> 24);
}

uint64_t bytes_to_uint64(unsigned char *b) {
  return uint64_t(b[0]) | uint64_t(b[1]) << 8 | uint64_t(b[2]) << 16 |
    uint64_t(b[3]) << 24 | uint64_t(b[4]) << 32 | uint64_t(b[5]) << 40 |
    uint64_t(b[6]) << 48 | uint64_t(b[7]) << 56;
}

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

};
};
