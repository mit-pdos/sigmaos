message(STATUS "Building unix conn library")

add_library(io_conn_unix STATIC unix.cc)

target_link_libraries(io_conn_unix PUBLIC
  io_conn
  serr
  util_log
)

target_include_directories(io_conn_unix PUBLIC ${CMAKE_SOURCE_DIR})
