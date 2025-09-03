#pragma once

#include <google/protobuf/message.h>
#include <google/protobuf/util/json_util.h>
#include <proc/proc.pb.h>

#include <expected>

namespace sigmaos {
namespace proc {

enum Tstatus : uint32_t {
  StatusOK = 1,
  StatusEvicted = 2,  // killed
  StatusErr = 3,
  StatusFatal =
      4,  // to indicate to groupmgr that a proc shouldn't be restarted

  // for testing purposes, meaning sigma doesn't know what happened
  // to proc; machine might have crashed.
  CRASH = 5,
};

};
};  // namespace sigmaos
