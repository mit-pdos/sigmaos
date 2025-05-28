#include <util/perf/perf.h>

#include <chrono>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>

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

google::protobuf::Timestamp GetCurrentTime() {
  auto current_unix_time_us = std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count();
  google::protobuf::Timestamp ts;
  ts.set_seconds(current_unix_time_us / 1000000);
  ts.set_nanos(((int64_t) current_unix_time_us % 1000000) * 1000);
  return ts;
}
