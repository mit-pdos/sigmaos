message(STATUS "Building spawn-latency-cpp proc")

#include_directories(proxy/sigmap/)

add_executable(spawn-latency-cpp
  main.cc
)
target_link_libraries(spawn-latency-cpp PRIVATE sigmap_proxy)
