#pragma once

#include <google/protobuf/timestamp.pb.h>
#include <google/protobuf/util/time_util.h>
#include <proc/proc.h>
#include <sigmap/types.h>
#include <util/log/log.h>

#include <format>
#include <fstream>

extern google::protobuf::Timestamp epoch;

void LogExecLatency(sigmaos::sigmap::types::Tpid pid,
                    google::protobuf::Timestamp spawn_time, std::string msg);

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid,
                     google::protobuf::Timestamp spawn_time,
                     google::protobuf::Timestamp op_start, std::string msg);

double LatencyMS(google::protobuf::Timestamp start);

google::protobuf::Timestamp GetCurrentTime();

namespace sigmaos {
namespace util::perf {

// Selector constants
const std::string TPT = "_TPT";

// Performance constants
const int SAMPLE_HZ = 50;
const int N_SECS_PREALLOC = 40;

// Path constants
const std::string PERF_OUTPUT_BASE_PATH = "/tmp/sigmaos-perf/";

class Perf {
 public:
  Perf(std::shared_ptr<sigmaos::proc::ProcEnv> pe, std::string selector)
      : _selector(selector), _pe(pe), _tpt(false), _tpts(), _times() {
    if (sigmaos::util::common::ContainsLabel(_pe->GetPerf(), _selector + TPT)) {
      setup_tpt(SAMPLE_HZ);
    }
  }
  ~Perf() { Done(); }

  // Record a throughput performance event
  void TptTick(double tpt);
  // Stop recording & save results
  void Done();

 private:
  std::mutex _mu;
  std::string _selector;
  std::shared_ptr<sigmaos::proc::ProcEnv> _pe;
  bool _tpt;
  std::vector<double> _tpts;
  std::vector<google::protobuf::Timestamp> _times;
  std::ofstream _tpt_file;
  double _tpt_sample_hz;

  void setup_tpt(int sample_hz);
  void teardown_tpt();
};

};  // namespace util::perf
};  // namespace sigmaos
