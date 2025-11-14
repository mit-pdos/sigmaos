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
protoc -I=. --cpp_out="./cpp/apps/cache/proto" --proto_path apps/cache/proto get.proto
protoc -I=. --cpp_out=./cpp util/tracing/proto/tracing.proto

cd cpp

# Make a build directory
mkdir -p build

# Generate build files
cd build
cmake ..
# Uncomment below to compile with debug symbols
#ccache -c && cmake -DCMAKE_BUILD_TYPE=RelWithDebInfo ..
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Run the build
make -j$(nproc)
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy to bins
cd $ROOT
USERBUILD=$ROOT/cpp/build/user
for p in $USERBUILD/* ; do
  name=$(basename $p)
  # Skip non-directories, and CMakefiles directory
  if ! [ -d $p ] || [[ "$name" == "CMakeFiles" ]] ; then
    continue
  fi
  # Copy to userbin
  cp $p/$name $USERBIN/$name-v$VERSION
done
