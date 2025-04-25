#pragma once

#include <cstdint>

namespace sigmaos {
namespace util::codec {

// Decoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=90;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
uint32_t bytes_to_uint32(unsigned char *b);

// Encoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=96;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
void uint32_to_bytes(unsigned char *b, uint32_t i);

// Decoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=115;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a?q=decodeFast&ss=go%2Fgo
uint64_t bytes_to_uint64(unsigned char *b);

// Encoding code taken from
// https://cs.opensource.google/go/go/+/master:src/encoding/binary/binary.go;l=122;drc=fca5832607d7c1afa20b82ca00ba4a27e28c0d0a;bpv=0;bpt=1
void uint64_to_bytes(unsigned char *b, uint64_t i);

};
};
