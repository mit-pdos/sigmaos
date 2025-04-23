message(STATUS "Building sigmap proto library")

add_library(sigmap_proto STATIC sigmap.pb.cc)

target_include_directories(sigmap_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
