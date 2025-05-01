message(STATUS "Building demux library")

add_library(io_demux STATIC
  demux.cc
  callmap.cc
)

target_link_libraries(io_demux PUBLIC
  util_log
  io_transport
)

target_include_directories(io_demux PUBLIC ${CMAKE_SOURCE_DIR})
