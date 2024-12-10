#!/bin/bash

# Note: order is important.

for P in sigmap proc example_echo_server ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in session dialproxy rpc kernel sched/besched sched/lcsched sched/msched sched/msched/proc realm proxy/db util/k8s util/tracing apps/cache apps/kv apps/hotel apps/socialnetwork proxy/sigmap chunk apps/imgresize proxy/mongo ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
