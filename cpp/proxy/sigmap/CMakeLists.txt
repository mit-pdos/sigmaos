message(STATUS "Building sigmap proxy library")

add_subdirectory(proto)

add_library(sigmap_proxy STATIC sigmap.cc)

target_link_libraries(sigmap_proxy PUBLIC
  io_conn
  io_frame
  io_transport
  io_demux
)

target_include_directories(sigmap_proxy PUBLIC ${CMAKE_SOURCE_DIR})
