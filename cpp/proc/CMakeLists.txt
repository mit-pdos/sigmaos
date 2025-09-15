message(STATUS "Building proc proto library")

add_library(proc STATIC proc.cc)

target_link_libraries(proc PUBLIC
  sigmap_proto
)

add_library(proc_proto STATIC proc.pb.cc)

target_link_libraries(proc_proto PUBLIC
  sigmap_proto
  absl::hash
  absl::status
)

target_include_directories(proc_proto PUBLIC ${CMAKE_SOURCE_DIR})
target_include_directories(proc PUBLIC ${CMAKE_SOURCE_DIR})
