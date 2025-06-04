message(STATUS "Building cache library")

add_subdirectory(proto)

add_library(app_cache STATIC
  clnt.cc
  cache.cc
)

target_link_libraries(app_cache PUBLIC
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
  rpc
  rpc_spchannel
  epcache_proto
  util_tracing_proto
  cache_proto
)

target_include_directories(app_cache PUBLIC ${CMAKE_SOURCE_DIR})
