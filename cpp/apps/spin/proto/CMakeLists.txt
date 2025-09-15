message(STATUS "Building spin proto library")

add_library(spin_proto STATIC spin.pb.cc)

target_include_directories(spin_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
