message(STATUS "Building frame library")

add_library(io_frame STATIC frame.cc)

target_include_directories(io_frame PUBLIC ${CMAKE_SOURCE_DIR})
