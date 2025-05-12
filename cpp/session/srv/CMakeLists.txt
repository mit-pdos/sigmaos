message(STATUS "Building sess_srv library")

add_library(sess_srv STATIC
  srv.cc
)

target_link_libraries(sess_srv PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(sess_srv PUBLIC ${CMAKE_SOURCE_DIR})
