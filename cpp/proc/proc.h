#pragma once

#include <expected>

#include <google/protobuf/message.h>
#include <google/protobuf/util/json_util.h>

#include <proc/proc.pb.h>

namespace sigmaos {
namespace proc {

class ProcEnv;

std::shared_ptr<ProcEnv> GetProcEnv();

class ProcEnv {
  public:
  ProcEnv(std::string pe_str) {
    auto res = google::protobuf::json::JsonStringToMessage(pe_str, &_proto);
    if (!res.ok()) {
      throw std::runtime_error(std::format("Error parse proc env str: {}", pe_str));
    }
  }
  ~ProcEnv() {}

  ProcEnvProto *GetProto() { return &_proto; }
  std::string String() { return _proto.ShortDebugString(); }

  std::string GetRealm() { return _proto.realmstr(); }

  private:
  ProcEnvProto _proto;
};

};
};
