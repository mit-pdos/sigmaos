#pragma once

#include <util/log/log.h>

namespace sigmaos {
namespace apps::cache {

const uint32_t NSHARD = 1009;

uint32_t key2shard(const std::string &key);
uint32_t key2server(const std::string &key, uint32_t nserver);

};  // namespace apps::cache
};  // namespace sigmaos
