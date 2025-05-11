message(STATUS "Building spawn-latency-cpp proc")

add_executable(echo-srv-cpp
  main.cc
)

target_link_libraries(echo-srv-cpp PRIVATE 
  io_conn_tcp
  util_log
  sigmap_proxy
)
