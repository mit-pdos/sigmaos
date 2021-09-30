#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

$DIR/start-all.sh

LEADER_ADDR=

# Get the address of the leader
while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  if [[ $hostname == $LEADER ]]; then
    LEADER_ADDR=$addr
  fi
done < $DIR/$SERVERS

ssh -i $DIR/keys/cloudlab-sigmaos $USER@$LEADER_ADDR <<ENDSSH

cd ulambda
./bin/user/microbenchmarks > benchmarks/results/microbenchmarks_${N_REPLICAS}_replicas.txt 2>&1
cat benchmarks/results/microbenchmarks_${N_REPLICAS}_replicas.txt

ENDSSH

scp -r -i $DIR/keys/cloudlab-sigmaos $USER@$LEADER_ADDR:~/ulambda/benchmarks/results/* $DIR/../benchmarks/results

$DIR/stop-all.sh
