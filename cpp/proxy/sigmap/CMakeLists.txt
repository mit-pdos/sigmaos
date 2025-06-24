message(STATUS "Building sigmap proxy library")

add_subdirectory(proto)

add_library(sigmap_proxy STATIC
  sigmap.cc
  delegated_rpc.cc
)

target_link_libraries(sigmap_proxy PUBLIC
  serr
  util_log
  util_perf
  rpc 
  proc
  proc_proto
  sigmap_proxy_proto
  io_conn
  io_conn_unix
  io_frame
  io_transport
  io_demux
)

target_include_directories(sigmap_proxy PUBLIC ${CMAKE_SOURCE_DIR})
