message(STATUS "Building net library")

add_library(io_net STATIC
  clnt.cc
  srv.cc
)

target_link_libraries(io_net PUBLIC
  serr
  util_log
  io_transport_internal
  io_conn_tcp
  io_frame
  io_iovec
)

target_include_directories(io_net PUBLIC ${CMAKE_SOURCE_DIR})
