#!/bin/bash

# Note: order is important.
for P in tracing ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/proto/$P.proto
done

for PP in cache hotel rpcbench ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
