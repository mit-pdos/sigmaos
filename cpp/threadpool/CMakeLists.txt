message(STATUS "Building threadpool library")

add_library(threadpool STATIC
  threadpool.cc
)

target_link_libraries(threadpool PUBLIC
  util_log
)

target_include_directories(threadpool PUBLIC ${CMAKE_SOURCE_DIR})
