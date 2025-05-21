#include <proc/proc.h>

namespace sigmaos {
namespace proc {

std::shared_ptr<ProcEnv> _env;

std::shared_ptr<ProcEnv> GetProcEnv() {
  if (_env) {
    return _env;
  }
  std::string pe_str(std::getenv("SIGMACONFIG"));
  if (pe_str.length() == 0) {
    fatal("Empty proc env");
  }
  _env = std::make_shared<ProcEnv>(pe_str);
  return _env;
}

google::protobuf::Timestamp GetExecTime() {
  google::protobuf::Timestamp exec_time;
  google::protobuf::util::TimeUtil::FromString(std::getenv("SIGMA_EXEC_TIME_PB"), &exec_time);
  return exec_time;
}

};
};
