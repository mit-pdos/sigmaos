message(STATUS "Building demux internal library")

add_library(io_demux_internal STATIC
  callmap.cc
)

target_link_libraries(io_demux_internal PUBLIC
  serr
  io_transport
)

target_include_directories(io_demux_internal PUBLIC ${CMAKE_SOURCE_DIR})
