message(STATUS "Building serr library")

add_library(serr STATIC serr.cc)

target_include_directories(serr PUBLIC ${CMAKE_SOURCE_DIR})
