message(STATUS "Building spawn-latency-cpp proc")

add_executable(spin-srv-cpp
  main.cc
)

target_link_libraries(spin-srv-cpp PRIVATE 
  io_net
  io_conn_tcp
  io_conn_unix
  util_log
  util_perf
  sigmap_proxy
  spin_proto
  app_spin
  rpc
)
