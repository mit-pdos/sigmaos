#!/bin/bash

# Note: order is important.

for P in sigmap proc ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in session dialproxy rpc ft/lease kernel sched/besched sched/lcsched sched/msched sched/msched/proc realm proxy/db util/k8s util/tracing apps/cache apps/kv/repl apps/kv apps/hotel apps/socialnetwork proxy/sigmap sched/msched/proc/chunk apps/imgresize proxy/mongo example/example_echo_server spproto/srv; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
