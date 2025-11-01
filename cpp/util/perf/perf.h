#pragma once

#include <google/protobuf/timestamp.pb.h>
#include <google/protobuf/util/time_util.h>
#include <proc/proc.h>
#include <sigmap/types.h>
#include <util/log/log.h>

#include <format>
#include <fstream>

extern google::protobuf::Timestamp epoch;

void LogRuntimeInitLatency(sigmaos::sigmap::types::Tpid pid,
                           google::protobuf::Timestamp spawn_time);

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid,
                     google::protobuf::Timestamp spawn_time,
                     google::protobuf::Timestamp op_start, std::string msg);

double LatencyMS(google::protobuf::Timestamp start);

google::protobuf::Timestamp GetCurrentTime();

namespace sigmaos {
namespace util::perf {

// Selector constants
const std::string TPT = "_TPT";
const std::string CPU = "_CPU";

// Performance constants
const int SAMPLE_HZ = 100;
const int N_SECS_PREALLOC = 40;

// Path constants
const std::string PERF_OUTPUT_BASE_PATH = "/tmp/sigmaos-perf/";

class Perf {
 public:
  Perf(std::shared_ptr<sigmaos::proc::ProcEnv> pe, std::string selector)
      : _selector(selector),
        _pe(pe),
        _pid(getpid()),
        _tpt(false),
        _tpts(),
        _tpt_times(),
        _cpu_util_times(),
        _cpu_util_active(false),
        _done(false) {
    if (sigmaos::util::common::ContainsLabel(_pe->GetPerf(), _selector + TPT)) {
      setup_tpt(SAMPLE_HZ);
    }
    if (sigmaos::util::common::ContainsLabel(_pe->GetPerf(), _selector + CPU)) {
      setup_cpu_util(SAMPLE_HZ);
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
  pid_t _pid;
  bool _tpt;
  std::vector<double> _tpts;
  std::vector<google::protobuf::Timestamp> _tpt_times;
  std::vector<google::protobuf::Timestamp> _cpu_util_times;
  std::ofstream _tpt_file;
  double _tpt_sample_hz;

  // CPU utilization monitoring
  bool _cpu_util_active;
  std::atomic<bool> _done;
  std::thread _cpu_util_thread;
  std::ofstream _cpu_util_file;
  std::string _proc_stat_path;
  std::vector<double> _cpu_cycles_busy;
  std::vector<double> _cpu_cycles_total;
  std::vector<double> _cpu_util_pct;

  void setup_tpt(int sample_hz);
  void teardown_tpt();
  void setup_cpu_util(int sample_hz);
  void teardown_cpu_util();
  void monitor_cpu_util(int sample_hz);
};

};  // namespace util::perf
};  // namespace sigmaos
