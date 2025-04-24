message(STATUS "Building conn library")

add_library(io_conn STATIC conn.cc)

target_include_directories(io_conn PUBLIC ${CMAKE_SOURCE_DIR})
