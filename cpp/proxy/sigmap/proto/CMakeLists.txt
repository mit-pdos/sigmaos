message(STATUS "Building sigmap proxy proto library")

add_library(sigmap_proxy_proto STATIC spproxy.pb.cc)

target_include_directories(sigmap_proxy_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
