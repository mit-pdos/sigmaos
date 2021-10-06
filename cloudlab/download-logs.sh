#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

mkdir -p $DIR/../benchmarks/results/logs

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  scp -i $DIR/keys/cloudlab-sigmaos $USER@$addr:~/ulambda/realmd.out $DIR/../benchmarks/results/logs/${hostname}_${N_REPLICAS}_replicas.out
done < $DIR/$SERVERS
