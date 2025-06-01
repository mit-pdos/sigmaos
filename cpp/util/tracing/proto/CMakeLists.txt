message(STATUS "Building tracing proto library")

add_library(util_tracing_proto STATIC tracing.pb.cc)

target_link_libraries(util_tracing_proto PUBLIC ${Protobuf_LIBRARIES})

target_include_directories(util_tracing_proto PRIVATE 
  ${CMAKE_SOURCE_DIR}
  ${Protobuf_INCLUDE_DIRS}
)
