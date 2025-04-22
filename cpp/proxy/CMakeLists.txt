message(STATUS "Building proxy libraries")

add_library(sigmap_proxy STATIC sigmap.cc)

target_include_directories(sigmap_proxy PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
