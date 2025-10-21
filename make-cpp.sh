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

# Compile protobufs
for P in sigmap proc ; do
  echo "protoc (cpp) $P"
  protoc -I=. --cpp_out=./cpp $P/$P.proto
done

for PP in proxy/sigmap ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc (cpp) $P cpp"
    protoc -I=. --cpp_out=./cpp $P
  done
done

for PP in rpc ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc (cpp) $P cpp"
    protoc -I=. --cpp_out=./cpp $P
  done
done

protoc -I=. --cpp_out="./cpp/apps/echo/proto" --proto_path example/example_echo_server/proto example_echo_server.proto
protoc -I=. --cpp_out="./cpp/apps/spin/proto" --proto_path apps/spin/proto spin.proto
protoc -I=. --cpp_out="./cpp/apps/cossim/proto" --proto_path apps/cossim/proto cossim.proto
protoc -I=. --cpp_out="./cpp/apps/epcache/proto" --proto_path apps/epcache/proto epcache.proto
protoc -I=. --cpp_out="./cpp/apps/cache/proto" --proto_path apps/cache/proto cache.proto
protoc -I=. --cpp_out=./cpp util/tracing/proto/tracing.proto

cd cpp

# Make a build directory
mkdir -p build

# Generate build files
cd build
cmake ..
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Run the build (builds everything: user procs, wasm-runtime, and .wasm modules)
make -j$(nproc)
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy to bins
cd $ROOT
KERNELBIN=$ROOT/bin/kernel
WASMBIN=$ROOT/bin/wasm
mkdir -p $KERNELBIN
mkdir -p $WASMBIN

# Copy wasm-runtime to kernel bin 
WASMRTBUILD=$ROOT/cpp/build/wasm-runtime
if [ -f $WASMRTBUILD/wasm-runtime ]; then
  cp $WASMRTBUILD/wasm-runtime $KERNELBIN/wasm-runtime
  cp $WASMRTBUILD/wasm-runtime $USERBIN/wasm-runtime-v$VERSION
wasm_runtime_bin=$KERNELBIN/wasm-runtime
imgresize_wasm_file=$WASMBIN/imgresize-wasm.wasm
input_file=/home/sigmaos/images/input/1.jpg
output_file=/home/sigmaos/images/output/1.jpg
spawn $wasm_runtime_bin $imgresize_wasm_file name/s3/~any/mysigmaos/images/input/1.jpg name/s3/~local/mysigmaos/images/output/1.jpg 1
  echo "Copied wasm-runtime to $KERNELBIN/wasm-runtime"
  echo "Copied wasm-runtime to $USERBIN/wasm-runtime-v$VERSION"
fi

# Copy user binaries
USERBUILD=$ROOT/cpp/build/user
for p in $USERBUILD/* ; do
  name=$(basename $p)
  # Skip non-directories, and CMakefiles directory
  if ! [ -d $p ] || [[ "$name" == "CMakeFiles" ]] ; then
    continue
  fi
  # Copy to userbin with version
  cp $p/$name $USERBIN/$name-v$VERSION
done

# Copy WASM modules to bin/wasm and bin/user (for binfs access)
WASMBUILD=$ROOT/cpp/build/wasm
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
      echo "Copied $wasm_name to $USERBIN/$wasm_name (for binfs)"s
    fi
  done
done
