#!/bin/bash

# Note: order is important.

for P in sigmap sessp ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in tracing cache kv hotel socialnetwork rpcbench ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
