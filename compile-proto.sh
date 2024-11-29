#!/bin/bash

# Note: order is important.

for P in sigmap sessp proc example_echo_server ; do
  echo "protoc $P"
  protoc -I=. --go_out=../ $P/$P.proto
done

for PP in dialproxy rpc kernelsrv sched/besched sched/lcsched sched/msched sched/msched/proc realm db util/k8s tracing apps/cache apps/kv apps/hotel apps/socialnetwork spproxy chunk apps/imgresize mongo lazypagessrv ; do
  for P in $PP/proto/*.proto ; do
    echo "protoc $P"
    protoc -I=. --go_out=../ $P
  done
done
