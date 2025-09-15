message(STATUS "Building logging library")

add_library(util_log STATIC log.cc)

target_link_libraries(util_log PUBLIC
  util_common
  spdlog::spdlog
)

target_include_directories(util_log PUBLIC ${CMAKE_SOURCE_DIR})
