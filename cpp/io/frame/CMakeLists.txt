message(STATUS "Building frame library")

add_library(io_frame STATIC frame.cc)

target_link_libraries(io_frame PUBLIC io_conn)

target_include_directories(io_frame PUBLIC ${CMAKE_SOURCE_DIR})
