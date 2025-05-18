message(STATUS "Building echo app library")

add_subdirectory(proto)

add_library(app_echo STATIC
  srv.cc
)

target_link_libraries(app_echo PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(app_echo PUBLIC ${CMAKE_SOURCE_DIR})
