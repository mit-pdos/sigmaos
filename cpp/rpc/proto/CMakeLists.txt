message(STATUS "Building rpc proto library")

add_library(rpc_proto STATIC rpc.pb.cc)

target_include_directories(rpc_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
