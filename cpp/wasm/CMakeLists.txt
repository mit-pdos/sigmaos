message(STATUS "Building wasm host")

add_executable(wasm_host
  host.cpp
)

# Find WasmEdge
find_library(WASMEDGE_LIB wasmedge PATHS $ENV{HOME}/.wasmedge/lib REQUIRED)
find_path(WASMEDGE_INCLUDE wasmedge/wasmedge.h PATHS $ENV{HOME}/.wasmedge/include REQUIRED)

target_include_directories(wasm_host PRIVATE 
  ${WASMEDGE_INCLUDE}
  ${CMAKE_SOURCE_DIR}
)

target_link_libraries(wasm_host PRIVATE
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
set_target_properties(wasm_host PROPERTIES
  INSTALL_RPATH "$ENV{HOME}/.wasmedge/lib"
  BUILD_WITH_INSTALL_RPATH TRUE
)

# Build WASM modules using wasi-sdk
find_program(WASI_CXX /opt/wasi-sdk/bin/clang++)

if(WASI_CXX)
  message(STATUS "Found wasi-sdk at ${WASI_CXX}")

  # Custom command to build hello.wasm
  add_custom_command(
    OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/hello.wasm
    COMMAND ${WASI_CXX}
      -O2
      --sysroot=/opt/wasi-sdk/share/wasi-sysroot
      -Wall
      -o ${CMAKE_CURRENT_BINARY_DIR}/hello.wasm
      ${CMAKE_CURRENT_SOURCE_DIR}/user/hello-world/hello.cpp
    DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/user/hello-world/hello.cpp
    COMMENT "Building hello.wasm with wasi-sdk"
  )

  # Create a custom target
  add_custom_target(wasm_hello ALL
    DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/hello.wasm
  )
else()
  message(WARNING "wasi-sdk not found at /opt/wasi-sdk/bin/clang++ - skipping WASM module builds")
endif()