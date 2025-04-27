message(STATUS "Building proc proto library")

add_library(sigmap_proc_proto STATIC proc.pb.cc)

target_link_libraries(sigmap_proc_proto PUBLIC
  sigmap_proto
  absl::hash
)

target_include_directories(sigmap_proc_proto PUBLIC ${CMAKE_SOURCE_DIR})
