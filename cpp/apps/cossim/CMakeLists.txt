message(STATUS "Building cossim app library")

add_subdirectory(proto)

add_library(app_cos_sim STATIC
  srv.cc
)

target_link_libraries(app_cos_sim PUBLIC
  serr
  util_log
  io_demux
  io_iovec
)

target_include_directories(app_cos_sim PUBLIC ${CMAKE_SOURCE_DIR})
