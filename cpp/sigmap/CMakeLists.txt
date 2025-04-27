message(STATUS "Building sigmap proto library")

add_library(sigmap_proto STATIC sigmap.pb.cc)

target_link_libraries(sigmap_proto PUBLIC ${Protobuf_LIBRARIES})

target_include_directories(sigmap_proto PUBLIC 
  ${Protobuf_INCLUDE_DIRS}
  ${CMAKE_SOURCE_DIR}
)
