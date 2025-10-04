#!/bin/bash

usage() {
  echo "Usage: $0 [--version VERSION]" 1>&2
}

VERSION="1.0"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --version)
    shift
    VERSION="$1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

ROOT=$(pwd)
USERBIN=$ROOT/bin/user
WASMBIN=$ROOT/bin/wasm

# Ensure output directories exist
mkdir -p $USERBIN
mkdir -p $WASMBIN

cd cpp

# Make a build directory
mkdir -p build_wasm

# Generate build files
cd build_wasm
cmake ..
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Build only .wasm module targets (wasm-runtime is built by cpp-builder)
make -j$(nproc) hello-world-wasm
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy WASM modules to bin/wasm and bin/user (for binfs access)
cd $ROOT
WASMBUILD=$ROOT/cpp/build_wasm/wasm
WASMUSER=$WASMBUILD/user
for p in $WASMUSER/* ; do
  name=$(basename $p)
  # Skip non-directories and CMakeFiles directory
  if ! [ -d $p ] || [[ "$name" == "CMakeFiles" ]] ; then
    continue
  fi
  # Look for .wasm files in the directory
  for wasm_file in $p/*.wasm ; do
    if [ -f "$wasm_file" ]; then
      wasm_name=$(basename $wasm_file)
      cp $wasm_file $WASMBIN/$wasm_name
      echo "Copied $wasm_name to $WASMBIN/$wasm_name"
      # Also copy to bin/user so it's accessible via /mnt/binfs
      cp $wasm_file $USERBIN/$wasm_name
      echo "Copied $wasm_name to $USERBIN/$wasm_name (for binfs)"
    fi
  done
done
