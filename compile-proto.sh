#!/bin/bash

# Note: order is important.

for P in sigmap sessp proc ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in netsigma rpc kernelsrv keysrv procqsrv lcschedsrv schedsrv uprocsrv realmsrv dbsrv k8sutil tracing cache kv hotel socialnetwork rpcbench sigmaclntsrv chunk ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
