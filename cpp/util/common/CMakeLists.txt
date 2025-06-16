message(STATUS "Building common util library")

add_library(util_common STATIC util.cc)

target_link_libraries(util_common PUBLIC)

target_include_directories(util_common PUBLIC ${CMAKE_SOURCE_DIR})
