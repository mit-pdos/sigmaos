message(STATUS "Building transport library")

add_library(io_transport STATIC
  transport.cc
  callmap.cc
)

target_link_libraries(io_transport PUBLIC
  util_log
  io_frame
)

target_include_directories(io_transport PUBLIC ${CMAKE_SOURCE_DIR})
