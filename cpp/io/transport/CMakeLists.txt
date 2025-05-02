message(STATUS "Building transport library")

add_subdirectory(internal)

add_library(io_transport STATIC
  transport.cc
)

target_link_libraries(io_transport PUBLIC
  serr
  util_log
  io_transport_internal
  io_frame
)

target_include_directories(io_transport PUBLIC ${CMAKE_SOURCE_DIR})
