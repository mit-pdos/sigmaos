message(STATUS "Building sigmap proxy library")

add_library(sigmap_proxy STATIC sigmap.cc)

target_include_directories(sigmap_proxy PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
