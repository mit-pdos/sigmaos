#include <util/common/util.h>

namespace sigmaos {
namespace util::common {

bool ContainsLabel(std::string env_var, std::string label) {
  bool contains_label = false;
  int nextpos = env_var.find(label);
  while (nextpos != std::string::npos) {
    // Save current pos
    int pos = nextpos;
    // Advance nextpos
    nextpos = env_var.find(label, pos + 1);
    // Check that the label is delimited on either side. If not, continue
    if (!(pos == 0 || env_var[pos - 1] == ';')) {
      continue;
    }
    if (!(pos + label.length() == env_var.length() || env_var[pos + label.length()] == ';')) {
      continue;
    }
    contains_label = true;
    break;
  }
  return contains_label;
}

};
};
