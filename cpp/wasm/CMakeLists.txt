message(STATUS "Building WASM components")

# Build the runtime host executable
add_subdirectory(runtime)

# Build WASM user modules
add_subdirectory(user)
