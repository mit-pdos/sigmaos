#include <util/perf/perf.h>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>

google::protobuf::Timestamp epoch = google::protobuf::util::TimeUtil::GetEpoch();

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid, google::protobuf::Timestamp spawn_time, google::protobuf::Timestamp op_start, std::string msg) {
  auto t = google::protobuf::util::TimeUtil::GetCurrentTime();
  double st_lat = 0;
  double op_lat = 0;
  // If spawn time was set, calculate latency
  if (spawn_time.seconds() != epoch.seconds() || spawn_time.nanos() != epoch.nanos()) {
    st_lat = (double) (t.seconds() - spawn_time.seconds()) + ((double) t.nanos() - spawn_time.nanos()) / (1E9);
  }
  // If op time was set, calculate latency
  if (op_start.seconds() != epoch.seconds() || op_start.nanos() != epoch.nanos()) {
    op_lat = (double) (t.seconds() - op_start.seconds()) + ((double) t.nanos() - op_start.nanos()) / (1E9);
  }
  log(SPAWN_LAT, "[{}] {} op:{} sinceSpawn:{}", pid, msg, st_lat, "xxx");
}
