message(STATUS "Building sigmap iovec library")

add_library(io_iovec STATIC iovec.cc)

target_include_directories(io_iovec PUBLIC ${CMAKE_SOURCE_DIR})
