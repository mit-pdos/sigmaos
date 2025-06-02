message(STATUS "Building cossim-srv-cpp proc")

add_executable(cossim-srv-cpp
  main.cc
)

target_link_libraries(cossim-srv-cpp PRIVATE 
  io_net
  io_conn_tcp
  io_conn_unix
  util_log
  util_perf
  sigmap_proxy
  spin_proto
  app_spin
  rpc
  rpc_spchannel
  app_cossim
)
