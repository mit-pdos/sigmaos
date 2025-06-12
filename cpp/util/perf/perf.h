#pragma once

#include <format>

#include <google/protobuf/util/time_util.h>
#include <google/protobuf/timestamp.pb.h>

#include <util/log/log.h>
#include <sigmap/types.h>

extern google::protobuf::Timestamp epoch;

void LogSpawnLatency(sigmaos::sigmap::types::Tpid pid, google::protobuf::Timestamp spawn_time, google::protobuf::Timestamp op_start, std::string msg);

double LatencyMS(google::protobuf::Timestamp start);

google::protobuf::Timestamp GetCurrentTime();
