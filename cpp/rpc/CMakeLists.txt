message(STATUS "Building rpc library")

add_subdirectory(proto)

add_library(rpc STATIC rpc.cc)

target_link_libraries(rpc PUBLIC
  util_log
  io_demux
)

target_include_directories(rpc PUBLIC ${CMAKE_SOURCE_DIR})
