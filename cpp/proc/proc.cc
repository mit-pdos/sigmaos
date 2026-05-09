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

void ResetProcEnv() { _env = nullptr; }

google::protobuf::Timestamp GetExecTime() {
  auto unix_micros_str = std::getenv("SIGMA_EXEC_TIME");
  int64_t unix_micros = std::stoll(unix_micros_str);
  google::protobuf::Timestamp exec_time;
  exec_time.set_seconds(unix_micros / 1000000);
  exec_time.set_nanos((unix_micros % 1000000) * 1000);
  return exec_time;
}

std::pair<std::shared_ptr<TendpointProto>, bool> ProcEnv::GetCachedEndpoint(
    std::string &pn) {
  if (!_proto.cachedendpoints().contains(pn)) {
    return std::make_pair(nullptr, false);
  }
  auto ep = _proto.cachedendpoints().at(pn);
  return std::make_pair(std::make_shared<TendpointProto>(ep), true);
}

};  // namespace proc
};  // namespace sigmaos
