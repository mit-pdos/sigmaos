message(STATUS "Building spawn-latency-cpp proc")

add_executable(echo-srv-cpp
  main.cc
)

target_link_libraries(echo-srv-cpp PRIVATE 
  io_net
  io_conn_tcp
  io_conn_unix
  util_log
  sigmap_proxy
  echo_proto
  app_echo
  rpc
)
