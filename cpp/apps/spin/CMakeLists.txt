message(STATUS "Building spin app library")

add_subdirectory(proto)

add_library(app_spin STATIC
  srv.cc
)

target_link_libraries(app_spin PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(app_spin PUBLIC ${CMAKE_SOURCE_DIR})
