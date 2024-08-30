#!/bin/bash

DIR=$(dirname $0)
source $DIR/env.sh

echo "Setting up servers"
cat $DIR/servers.txt

OUT_DIR=/tmp/sigmaos-cloudlab-node-logs

mkdir -p $OUT_DIR

for s in $(cat $DIR/servers.txt | cut -d " " -f 2); do
  ./setup-k8s.sh $s > $OUT_DIR/$s-k8s-setup.out 2>&1 &
  sleep 10
done

wait

echo "Done setting up k8s on cluster"
