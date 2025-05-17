message(STATUS "Building echo proto library")

add_library(echo_proto STATIC example_echo_server.pb.cc)

target_include_directories(echo_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
