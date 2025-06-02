message(STATUS "Building cossim app library")

add_subdirectory(proto)

add_library(app_cossim STATIC
  srv.cc
)

target_link_libraries(app_cossim PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(app_cossim PUBLIC ${CMAKE_SOURCE_DIR})
