message(STATUS "Building WASM user modules")

# Find wasi-sdk compiler
find_program(WASI_CXX /opt/wasi-sdk/bin/clang++)

if(NOT WASI_CXX)
  message(WARNING "wasi-sdk not found at /opt/wasi-sdk/bin/clang++ - skipping WASM module builds")
  return()
endif()

message(STATUS "Found wasi-sdk at ${WASI_CXX}")

# Add subdirectories for each WASM user module
add_subdirectory(start-proc-wasm)
add_subdirectory(spawn-latency-wasm)
add_subdirectory(imgresize)
