message(STATUS "Building wasm runtime host")

add_executable(wasm-runtime
  main.cpp
)

# Find WasmEdge
find_library(WASMEDGE_LIB NAMES wasmedge PATHS $ENV{HOME}/.wasmedge/lib NO_DEFAULT_PATH REQUIRED)
find_path(WASMEDGE_INCLUDE wasmedge/wasmedge.h PATHS $ENV{HOME}/.wasmedge/include NO_DEFAULT_PATH REQUIRED)

target_include_directories(wasm-runtime PRIVATE
  ${WASMEDGE_INCLUDE}
  ${CMAKE_SOURCE_DIR}
)

target_link_libraries(wasm-runtime PRIVATE
  ${WASMEDGE_LIB}
  sigmap_proxy
  serr
  util_log
  util_perf
  rpc
  proc
  io_conn
  io_conn_unix
  io_frame
  io_transport
  io_demux
)

# Set rpath for WasmEdge library
set_target_properties(wasm-runtime PROPERTIES
  INSTALL_RPATH "$ENV{HOME}/.wasmedge/lib"
  BUILD_WITH_INSTALL_RPATH TRUE
)
