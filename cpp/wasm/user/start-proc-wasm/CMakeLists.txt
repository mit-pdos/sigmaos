message(STATUS "Building hello-world-wasm module")

# Custom command to build hello-world-wasm.wasm
add_custom_command(
  OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/hello-world-wasm.wasm
  COMMAND ${WASI_CXX}
    -O2
    --sysroot=/opt/wasi-sdk/share/wasi-sysroot
    -Wall
    -o ${CMAKE_CURRENT_BINARY_DIR}/hello-world-wasm.wasm
    ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  COMMENT "Building hello-world-wasm.wasm with wasi-sdk"
)

# Create a custom target
add_custom_target(hello-world-wasm ALL
  DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/hello-world-wasm.wasm
)
