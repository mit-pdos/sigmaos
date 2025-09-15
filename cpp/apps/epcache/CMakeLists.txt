message(STATUS "Building epcache library")

add_subdirectory(proto)

add_library(app_epcache STATIC clnt.cc)

target_link_libraries(app_epcache PUBLIC
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
)

target_include_directories(app_epcache PUBLIC ${CMAKE_SOURCE_DIR})
