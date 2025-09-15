message(STATUS "Building rpc_spchannel library")

add_library(rpc_spchannel STATIC
  spchannel.cc
)

target_link_libraries(rpc_spchannel PUBLIC
  serr
  util_log
  util_perf
  io_demux
  io_iovec
)

target_include_directories(rpc_spchannel PUBLIC ${CMAKE_SOURCE_DIR})
