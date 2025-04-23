message(STATUS "Building sigmap proxy library")

add_subdirectory(proto)

add_library(sigmap_proxy STATIC sigmap.cc)

target_include_directories(sigmap_proxy PUBLIC ${CMAKE_SOURCE_DIR})
