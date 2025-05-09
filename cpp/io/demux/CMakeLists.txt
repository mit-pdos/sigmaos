message(STATUS "Building demux library")

add_subdirectory(internal)

add_library(io_demux STATIC
  demux.cc
)

target_link_libraries(io_demux PUBLIC
  serr
  util_log
  io_demux_internal
  io_transport
)

target_include_directories(io_demux PUBLIC ${CMAKE_SOURCE_DIR})
