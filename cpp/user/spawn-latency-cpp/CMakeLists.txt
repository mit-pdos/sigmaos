message(STATUS "Building spawn-latency-cpp proc")

add_executable(spawn-latency-cpp
  main.cc
)

target_link_libraries(spawn-latency-cpp PRIVATE 
  util_log
  sigmap_proxy
)
