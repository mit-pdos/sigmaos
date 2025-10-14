message(STATUS "Building start-proc-wasm module")

# Custom command to build start-proc-wasm.wasm
add_custom_command(
  OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/start-proc-wasm.wasm
  COMMAND ${WASI_CXX}
    -O2
    --sysroot=/opt/wasi-sdk/share/wasi-sysroot
    -Wall
    -o ${CMAKE_CURRENT_BINARY_DIR}/start-proc-wasm.wasm
    ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  COMMENT "Building start-proc-wasm.wasm with wasi-sdk"
)

# Create a custom target
add_custom_target(start-proc-wasm ALL
  DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/start-proc-wasm.wasm
)
