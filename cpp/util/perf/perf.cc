#include <util/perf/perf.h>

#include <chrono>

#include <google/protobuf/util/time_util.h>

google::protobuf::Timestamp epoch = google::protobuf::util::TimeUtil::GetEpoch();

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid, google::protobuf::Timestamp spawn_time, google::protobuf::Timestamp op_start, std::string msg) {
  // Calculate current unix time in msec
  auto current_unix_time_us = std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count();
  double st_lat_ms = 0;
  double op_lat_ms = 0;
  // If spawn time was set, calculate latency
  if (spawn_time.seconds() != epoch.seconds() || spawn_time.nanos() != epoch.nanos()) {
    st_lat_ms = current_unix_time_us / 1000.0 - ((double) spawn_time.seconds() * 1000 + (double) spawn_time.nanos() / (1E6));
  }
  // If op time was set, calculate latency
  if (op_start.seconds() != epoch.seconds() || op_start.nanos() != epoch.nanos()) {
    op_lat_ms = current_unix_time_us / 1000.0 - ((double) op_start.seconds() * 1000 + (double) op_start.nanos() / (1E6));
  }
  log(SPAWN_LAT, "[{}] {} op:{:.3f}ms sinceSpawn:{:.3f}ms", pid, msg, op_lat_ms, st_lat_ms);
}

double LatencyMS(google::protobuf::Timestamp start) {
  auto current_unix_time_us = std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count();
  double lat_ms = current_unix_time_us / 1000.0 - ((double) start.seconds() * 1000 + (double) start.nanos() / (1E6));
  return lat_ms;
}

google::protobuf::Timestamp GetCurrentTime() {
  auto current_unix_time_us = std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count();
  google::protobuf::Timestamp ts;
  ts.set_seconds(current_unix_time_us / 1000000);
  ts.set_nanos(((int64_t) current_unix_time_us % 1000000) * 1000);
  return ts;
}

namespace sigmaos {
namespace util::perf {

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
	// _times.size() == _tpts.size() - 1
	if (LatencyMS(_times.back()) > (1000.0 / _tpt_sample_hz)) {
    _tpts.push_back(0.0);
    _times.push_back(GetCurrentTime());
	}

	// Increment the current tpt slot.
	_tpts.back() += tpt;
}

void Perf::Done() {
  teardown_tpt();
}

void Perf::setup_tpt(int sample_hz) {
  // Lock to make sure setup is atomic
  std::lock_guard<std::mutex> guard(_mu);
  _tpt = true;
  _tpt_sample_hz = (double) sample_hz;
  // Reserve capacity in the vectors to make sure resizing doesn't impact
  // performance.
  _times.reserve(N_SECS_PREALLOC * sample_hz);
  _tpts.reserve(N_SECS_PREALLOC * sample_hz);

  std::string pn = PERF_OUTPUT_BASE_PATH + _pe->GetPID() + "-tpt.out";

  // Initialize the output file stream
  _tpt_file = std::ofstream(pn, ::std::ios::out | std::ios::app);

  // Append the initial entry to the times & throughputs vectors
  _times.push_back(GetCurrentTime());
  _tpts.push_back(0.0);
}

void Perf::teardown_tpt() {
  // Lock to make sure teardown is atomic
  std::lock_guard<std::mutex> guard(_mu);

  if (_tpt) {
    _tpt = false;
    for (int i = 0; i < _times.size(); i++) {
      auto ts = _times.at(i);
      double tpt = _tpts.at(i);
      _tpt_file << ts.seconds() * 1000000 + ts.nanos() / 1000 << "us," << tpt << std::endl;
    }
    _tpt_file.close();
  }
}

};
};
