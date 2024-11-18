#!/bin/bash

# Note: order is important.

for P in sigmap sessp proc example_echo_server ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in netproxy rpc kernelsrv keysrv beschedsrv lcschedsrv schedsrv uprocsrv realmsrv dbsrv k8sutil tracing cache apps/kv apps/hotel apps/socialnetwork spproxysrv chunk apps/imgresize ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
