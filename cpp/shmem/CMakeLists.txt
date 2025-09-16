message(STATUS "Building shmem library")

add_library(shmem STATIC
  segment.cc
  shmem.cc
)

target_link_libraries(shmem PUBLIC
  serr
  util_log
)

target_include_directories(shmem PUBLIC ${CMAKE_SOURCE_DIR})
