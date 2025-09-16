#pragma once

#include <sys/types.h>

#include <cstdint>
#include <string>

namespace sigmaos {
namespace shmem {

uint32_t id2key(const std::string &key);

};  // namespace shmem
};  // namespace sigmaos
