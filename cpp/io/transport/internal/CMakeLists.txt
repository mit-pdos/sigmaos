message(STATUS "Building transport internal library")

add_library(io_transport_internal STATIC
  callmap.cc
)

target_link_libraries(io_transport_internal PUBLIC
  util_log
  io_transport
)

target_include_directories(io_transport_internal PUBLIC ${CMAKE_SOURCE_DIR})
