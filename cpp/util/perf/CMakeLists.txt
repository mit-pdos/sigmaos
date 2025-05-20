message(STATUS "Building perf library")

add_library(util_perf STATIC perf.cc)

target_link_libraries(util_perf PUBLIC
  util_log
)

target_include_directories(util_perf PUBLIC ${CMAKE_SOURCE_DIR})
