#include <util/perf/perf.h>

#include <chrono>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>

google::protobuf::Timestamp epoch = google::protobuf::util::TimeUtil::GetEpoch();

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid, google::protobuf::Timestamp spawn_time, google::protobuf::Timestamp op_start, std::string msg) {
  // Calculate current unix time in msec
  auto current_unix_time = std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::high_resolution_clock::now().time_since_epoch()).count();
  double st_lat = 0;
  double op_lat = 0;
  // If spawn time was set, calculate latency
  if (spawn_time.seconds() != epoch.seconds() || spawn_time.nanos() != epoch.nanos()) {
    st_lat = current_unix_time - ((double) spawn_time.seconds() * 1000 + (double) spawn_time.nanos() / (1E6));
  }
  // If op time was set, calculate latency
  if (op_start.seconds() != epoch.seconds() || op_start.nanos() != epoch.nanos()) {
    op_lat = current_unix_time - ((double) op_start.seconds() * 1000 + (double) op_start.nanos() / (1E6));
  }
  log(SPAWN_LAT, "[{}] {} op:{}ms sinceSpawn:{}ms", pid, msg, op_lat, st_lat);
}
