message(STATUS "Building rpc delegation library")

add_library(rpc_delegation STATIC
  cache.cc
)

target_link_libraries(rpc_delegation PUBLIC
  serr
  util_log
  ${Protobuf_LIBRARIES}
)

target_include_directories(rpc_delegation PUBLIC ${CMAKE_SOURCE_DIR})