#pragma once

#include <util/log/log.h>

namespace sigmaos {
namespace apps::cache {

const uint32_t NSHARD = 1009;

uint32_t key2shard(std::string key);

};
};
