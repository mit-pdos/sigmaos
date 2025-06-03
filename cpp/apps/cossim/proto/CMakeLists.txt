message(STATUS "Building cossim proto library")

add_library(cossim_proto STATIC cossim.pb.cc)

target_include_directories(cossim_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
