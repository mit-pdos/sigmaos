message(STATUS "Building imgresize-wasm module")

# Custom command to build imgresize-wasm.wasm
add_custom_command(
  OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/imgresize-wasm.wasm
  COMMAND ${WASI_CXX}
    -O2
    --sysroot=/opt/wasi-sdk/share/wasi-sysroot
    -Wall
    -Wno-unused-command-line-argument
    -o ${CMAKE_CURRENT_BINARY_DIR}/imgresize-wasm.wasm
    ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp
  DEPENDS ${CMAKE_CURRENT_SOURCE_DIR}/main.cpp ${CMAKE_CURRENT_SOURCE_DIR}/sigmaos.h
  COMMENT "Building imgresize-wasm.wasm with wasi-sdk"
)

# Create a custom target
add_custom_target(imgresize-wasm ALL
  DEPENDS ${CMAKE_CURRENT_BINARY_DIR}/imgresize-wasm.wasm
)
