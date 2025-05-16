message(STATUS "Building tcp conn library")

add_library(io_conn_tcp STATIC tcp.cc)

target_link_libraries(io_conn_tcp PUBLIC
  io_conn
  serr
  util_log
)

target_include_directories(io_conn_tcp PUBLIC ${CMAKE_SOURCE_DIR})
