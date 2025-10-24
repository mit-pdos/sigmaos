message(STATUS "Building demux library")

add_subdirectory(internal)

add_library(io_demux STATIC
  clnt.cc
  srv.cc
)

target_link_libraries(io_demux PUBLIC
  serr
  util_log
  util_metrics
  io_demux_internal
  io_transport
  threadpool
)

target_include_directories(io_demux PUBLIC ${CMAKE_SOURCE_DIR})
