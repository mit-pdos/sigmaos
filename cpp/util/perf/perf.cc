#include <google/protobuf/util/time_util.h>
#include <sched.h>
#include <unistd.h>
#include <util/perf/perf.h>

#include <chrono>
#include <fstream>
#include <sstream>
#include <string>
#include <thread>

google::protobuf::Timestamp epoch =
    google::protobuf::util::TimeUtil::GetEpoch();

void LogRuntimeInitLatency(sigmaos::sigmap::types::Tpid pid,
                           google::protobuf::Timestamp spawn_time) {
  auto exec_time = sigmaos::proc::GetExecTime();
  LogSpawnLatency(pid, spawn_time, exec_time, "Setup.RuntimeInit");
}

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid,
                     google::protobuf::Timestamp spawn_time,
                     google::protobuf::Timestamp op_start, std::string msg) {
  // Calculate current unix time in msec
  auto current_unix_time_us =
      std::chrono::duration_cast<std::chrono::microseconds>(
          std::chrono::high_resolution_clock::now().time_since_epoch())
          .count();
  double st_lat_ms = 0;
  double op_lat_ms = 0;
  // If spawn time was set, calculate latency
  if (spawn_time.seconds() != epoch.seconds() ||
      spawn_time.nanos() != epoch.nanos()) {
    st_lat_ms =
        current_unix_time_us / 1000.0 - ((double)spawn_time.seconds() * 1000 +
                                         (double)spawn_time.nanos() / (1E6));
  }
  // If op time was set, calculate latency
  if (op_start.seconds() != epoch.seconds() ||
      op_start.nanos() != epoch.nanos()) {
    op_lat_ms =
        current_unix_time_us / 1000.0 -
        ((double)op_start.seconds() * 1000 + (double)op_start.nanos() / (1E6));
  }
  log(SPAWN_LAT, "[{}] {} op:{:0.3f}ms sinceSpawn:{:0.3f}ms", pid, msg,
      op_lat_ms, st_lat_ms);
}

double LatencyMS(google::protobuf::Timestamp start) {
  auto current_unix_time_us =
      std::chrono::duration_cast<std::chrono::microseconds>(
          std::chrono::high_resolution_clock::now().time_since_epoch())
          .count();
  double lat_ms =
      current_unix_time_us / 1000.0 -
      ((double)start.seconds() * 1000 + (double)start.nanos() / (1E6));
  return lat_ms;
}

google::protobuf::Timestamp GetCurrentTime() {
  auto current_unix_time_us =
      std::chrono::duration_cast<std::chrono::microseconds>(
          std::chrono::high_resolution_clock::now().time_since_epoch())
          .count();
  google::protobuf::Timestamp ts;
  ts.set_seconds(current_unix_time_us / 1000000);
  ts.set_nanos(((int64_t)current_unix_time_us % 1000000) * 1000);
  return ts;
}

namespace sigmaos {
namespace util::perf {

// Helper function to get CPU time usage for a specific process
// Returns utime and stime from /proc/[pid]/stat
static std::pair<uint64_t, uint64_t> GetCPUTimePid(
    const std::string& stat_path) {
  uint64_t utime = 0;
  uint64_t stime = 0;

  std::ifstream stat_file(stat_path);
  if (!stat_file.is_open()) {
    ::log(ALWAYS, "Error: Could not open {}", stat_path);
    return {utime, stime};
  }

  std::string line;
  std::getline(stat_file, line);

  // Parse the stat file. Fields are space-separated.
  // utime is field 14 (index 13), stime is field 15 (index 14)
  std::istringstream iss(line);
  std::string field;
  for (int i = 0; i < 15; i++) {
    iss >> field;
    if (i == 13) {
      utime = std::stoull(field);
    } else if (i == 14) {
      stime = std::stoull(field);
    }
  }

  return {utime, stime};
}

// Record a throughput performance event
void Perf::TptTick(double tpt) {
  // If not recording throughput, bail out
  if (!_tpt) {
    return;
  }

  // Lock to make sure recording is atomic
  std::lock_guard<std::mutex> guard(_mu);

  // If it has been long enough since we started incrementing this slot, seal
  // it and move to the next slot. In this way, we always expect
  // _tpt_times.size() == _tpts.size() - 1
  if (LatencyMS(_tpt_times.back()) > (1000.0 / _tpt_sample_hz)) {
    _tpts.push_back(0.0);
    _tpt_times.push_back(GetCurrentTime());
  }

  // Increment the current tpt slot.
  _tpts.back() += tpt;
}

void Perf::Done() {
  teardown_tpt();
  teardown_cpu_util();
}

void Perf::monitor_cpu_util(int sample_hz) {
  int sleep_msecs = 1000 / sample_hz;
  uint64_t utime0, stime0, utime1, stime1;
  uint64_t total0, total1;

  // Get initial sample
  auto [init_utime, init_stime] = GetCPUTimePid(_proc_stat_path);
  utime0 = init_utime;
  stime0 = init_stime;
  total0 = utime0 + stime0;

  while (!_done.load()) {
    std::this_thread::sleep_for(std::chrono::milliseconds(sleep_msecs));

    auto [curr_utime, curr_stime] = GetCPUTimePid(_proc_stat_path);
    utime1 = curr_utime;
    stime1 = curr_stime;
    total1 = utime1 + stime1;

    double total_delta = static_cast<double>(total1 - total0);
    // For process-specific monitoring, all delta is "busy" time
    double util =
        100.0 * total_delta / (sleep_msecs * sysconf(_SC_CLK_TCK) / 1000.0);

    // Record number of cycles busy, total, and utilization percentage
    {
      std::lock_guard<std::mutex> guard(_mu);
      _cpu_cycles_busy.push_back(total_delta);
      _cpu_cycles_total.push_back(total_delta);
      _cpu_util_pct.push_back(util);
      _cpu_util_times.push_back(GetCurrentTime());
    }

    utime0 = utime1;
    stime0 = stime1;
    total0 = total1;
  }
}

void Perf::setup_tpt(int sample_hz) {
  // Lock to make sure setup is atomic
  std::lock_guard<std::mutex> guard(_mu);
  _tpt = true;
  _tpt_sample_hz = (double)sample_hz;
  // Reserve capacity in the vectors to make sure resizing doesn't impact
  // performance.
  _tpt_times.reserve(N_SECS_PREALLOC * sample_hz);
  _tpts.reserve(N_SECS_PREALLOC * sample_hz);

  std::string pn = PERF_OUTPUT_BASE_PATH + _pe->GetPID() + "-tpt.out";

  // Initialize the output file stream
  _tpt_file = std::ofstream(pn, ::std::ios::out | std::ios::app);

  // Append the initial entry to the times & throughputs vectors
  _tpt_times.push_back(GetCurrentTime());
  _tpts.push_back(0.0);
}

void Perf::teardown_tpt() {
  // Lock to make sure teardown is atomic
  std::lock_guard<std::mutex> guard(_mu);

  if (_tpt) {
    _tpt = false;
    for (int i = 0; i < _tpt_times.size(); i++) {
      auto ts = _tpt_times.at(i);
      double tpt = _tpts.at(i);
      _tpt_file << ts.seconds() * 1000000 + ts.nanos() / 1000 << "us," << tpt
                << std::endl;
    }
    _tpt_file.close();
  }
}

void Perf::setup_cpu_util(int sample_hz) {
  // Lock to make sure setup is atomic
  std::lock_guard<std::mutex> guard(_mu);

  _cpu_util_active = true;

  // Store the proc stat path to avoid recreating it on every sample
  _proc_stat_path = "/proc/" + std::to_string(_pid) + "/stat";

  // Reserve capacity in the vectors to make sure resizing doesn't impact
  // performance.
  _cpu_cycles_busy.reserve(N_SECS_PREALLOC * sample_hz);
  _cpu_cycles_total.reserve(N_SECS_PREALLOC * sample_hz);
  _cpu_util_pct.reserve(N_SECS_PREALLOC * sample_hz);
  _cpu_util_times.reserve(N_SECS_PREALLOC * sample_hz);

  std::string pn = PERF_OUTPUT_BASE_PATH + _pe->GetPID() + "-cpu.out";

  // Initialize the output file stream
  _cpu_util_file = std::ofstream(pn, std::ios::out | std::ios::app);

  // Start the monitoring thread
  _cpu_util_thread = std::thread(&Perf::monitor_cpu_util, this, sample_hz);
}

void Perf::teardown_cpu_util() {
  if (_cpu_util_active) {
    // Signal the thread to stop
    _done.store(true);

    // Wait for the thread to finish
    if (_cpu_util_thread.joinable()) {
      _cpu_util_thread.join();
    }

    // Lock to make sure teardown is atomic
    std::lock_guard<std::mutex> guard(_mu);

    _cpu_util_active = false;

    // Write all the collected data to the file
    for (size_t i = 0; i < _cpu_util_pct.size(); i++) {
      auto ts = _cpu_util_times.at(i);
      _cpu_util_file << ts.seconds() * 1000000 + ts.nanos() / 1000 << "us," << _cpu_util_pct[i] << "," << _cpu_cycles_busy[i] << ","
                     << _cpu_cycles_total[i] << std::endl;
    }
    _cpu_util_file.close();
  }
}

};  // namespace util::perf
};  // namespace sigmaos
