message(STATUS "Building demux library")

add_library(io_demux STATIC demux.cc)

target_include_directories(io_demux PUBLIC ${CMAKE_SOURCE_DIR})
