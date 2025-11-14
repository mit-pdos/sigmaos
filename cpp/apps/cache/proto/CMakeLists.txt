message(STATUS "Building cache proto library")

add_library(cache_proto STATIC
 cache.pb.cc
 get.pb.cc
 dump.pb.cc
)

target_link_libraries(cache_proto PUBLIC util_tracing_proto)

target_include_directories(cache_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
