#include <apps/cache/cache.h>

namespace sigmaos {
namespace apps::cache {

uint32_t key2shard(const std::string &key) {
  // fnv32a hash inspired by
  // https://cs.opensource.google/go/go/+/refs/tags/go1.24.3:src/hash/fnv/fnv.go;l=51
  uint32_t s = 2166136261;
  uint32_t prime32 = 16777619;
  for (int i = 0; i < key.size(); i++) {
    s ^= (uint32_t)key[i];
    s *= prime32;
  }
  return s % NSHARD;
}

uint32_t key2server(const std::string &key, uint32_t nserver) {
  uint32_t shard = key2shard(key);
  return shard % nserver;
}

};  // namespace apps::cache
};  // namespace sigmaos
