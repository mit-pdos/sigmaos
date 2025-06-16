#pragma once

#include <string>

namespace sigmaos {
namespace util::common {

// Returns true if the given SigmaOS environment variable contains the given
// label.
bool ContainsLabel(std::string env_var, std::string label);

};
};
