message(STATUS "Building conn library")

add_library(io_conn STATIC conn.cc)

target_link_libraries(io_conn PUBLIC
  util_codec
  util_log
)

target_include_directories(io_conn PUBLIC ${CMAKE_SOURCE_DIR})
