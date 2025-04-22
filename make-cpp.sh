#!/bin/bash

cd cpp

# Make a build directory
mkdir -p build

# Generate build files
cd build
cmake ..

# Run the build
make -j$(nproc)
