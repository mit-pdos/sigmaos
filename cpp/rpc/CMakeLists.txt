message(STATUS "Building rpc library")

add_subdirectory(proto)
add_subdirectory(spchannel)

add_library(rpc STATIC
  blob.cc
  clnt.cc
  srv.cc
)

target_link_libraries(rpc PUBLIC
  serr
  util_log
  util_perf
  io_demux
  io_iovec
)

target_include_directories(rpc PUBLIC ${CMAKE_SOURCE_DIR})
