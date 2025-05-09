message(STATUS "Building rpc library")

add_subdirectory(proto)

add_library(rpc STATIC rpc.cc)

target_link_libraries(rpc PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(rpc PUBLIC ${CMAKE_SOURCE_DIR})
