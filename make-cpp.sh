#!/bin/bash

# Compile protobufs
for P in sigmap proc ; do
  echo "protoc $P cpp"
  protoc -I=. --cpp_out=./cpp $P/$P.proto
done

for PP in proxy/sigmap ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P cpp"
    protoc -I=. --cpp_out=./cpp $P
  done
done

for PP in rpc ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P golang"
    protoc -I=. --cpp_out=./cpp $P
  done
done

cd cpp

# Make a build directory
mkdir -p build

# Generate build files
cd build
cmake ..

# Run the build
make -j$(nproc)
