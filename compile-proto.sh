#!/bin/bash

# TODO: add more
# Note: order is important.
for P in tracing ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/proto/$P.proto
done

for PP in hotel ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
