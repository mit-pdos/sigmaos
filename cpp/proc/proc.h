#pragma once

#include <expected>
#include <utility>

#include <google/protobuf/message.h>
#include <google/protobuf/util/json_util.h>
#include <google/protobuf/util/time_util.h>

#include <proc/proc.pb.h>
#include <util/log/log.h>
#include <sigmap/types.h>

namespace sigmaos {
namespace proc {

class ProcEnv;

std::shared_ptr<ProcEnv> GetProcEnv();
google::protobuf::Timestamp GetExecTime();

class ProcEnv {
  public:
  ProcEnv(std::string pe_str) {
    auto res = google::protobuf::util::JsonStringToMessage(pe_str, &_proto);
    if (!res.ok()) {
      fatal("Error parse proc env str: {}", pe_str);
    }
  }
  ~ProcEnv() {}

  ProcEnvProto *GetProto() { return &_proto; }
  std::string String() { return _proto.ShortDebugString(); }

  sigmaos::sigmap::types::Trealm GetRealm() { return _proto.realmstr(); }
  sigmaos::sigmap::types::Tpid GetPID() { return _proto.pidstr(); }
  sigmaos::sigmap::types::Tip GetOuterContainerIP() { return _proto.outercontaineripstr(); }
  google::protobuf::Timestamp GetSpawnTime() { return _proto.spawntimepb(); }
  std::string GetPerf() { return _proto.perf(); }
  bool GetDelegateInit() { return _proto.delegateinitflag(); }
  std::pair<std::shared_ptr<TendpointProto>, bool> GetCachedEndpoint(std::string &pn);

  private:
  ProcEnvProto _proto;
};

};
};
