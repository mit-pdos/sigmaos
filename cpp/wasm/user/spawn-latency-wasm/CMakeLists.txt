message(STATUS "Building spawn-latency-wasm module")

# Custom command to build spawn-latency-wasm.wasm
add_custom_command(
  OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/spawn-latency-wasm.wasm
  COMMAND ${WASI_CXX}
    -O2
    --sysroot=/opt/wasi-sdk/share/wasi-sysroot
    -Wall
    -o ${CMAKE_CURRENT_BINARY_DIR}/spawn-latency-wasm.wasm
    ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  COMMENT "Building spawn-latency-wasm.wasm with wasi-sdk"
)

# Create a custom target
add_custom_target(spawn-latency-wasm ALL
  DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/spawn-latency-wasm.wasm
)
