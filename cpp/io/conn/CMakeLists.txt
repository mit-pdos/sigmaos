message(STATUS "Building conn library")

add_library(io_conn STATIC conn.cc)

target_include_directories(io_conn PUBLIC ${CMAKE_SOURCE_DIR})

target_link_libraries(io_conn PUBLIC
  serr
)

add_subdirectory(unix)
