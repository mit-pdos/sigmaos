#include <shmem/shmem.h>

namespace sigmaos {
namespace shmem {

uint64_t id2key(const std::string &key) {
  // fnv64a hash inspired by
  // https://cs.opensource.google/go/go/+/refs/tags/go1.24.3:src/hash/fnv/fnv.go;l=65
  uint64_t s = 14695981039346656037LLU;
  uint64_t prime64 = 1099511628211;
  for (int i = 0; i < key.size(); i++) {
    s ^= (uint64_t)key[i];
    s *= prime64;
  }
  return s;
}

};  // namespace shmem
};  // namespace sigmaos
