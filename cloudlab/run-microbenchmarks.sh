#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

echo "Stopping replicas..."
$DIR/stop-all.sh
echo "Done stopping replicas..."

echo "Starting replicas..."
$DIR/start-all.sh
echo "Done starting replicas..."

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

echo "Running microbenchmarks..."
ssh -i $DIR/keys/cloudlab-sigmaos $USER@$LEADER_ADDR <<ENDSSH

ulimit -n 100000
export NAMED=$LEADER_ADDR:1111
cd ulambda
GOGC=off ./bin/user/microbenchmarks > benchmarks/results/microbenchmarks_${N_REPLICAS}_replicas.txt 2>&1
cat benchmarks/results/microbenchmarks_${N_REPLICAS}_replicas.txt

ENDSSH
echo "Done running microbenchmarks..."

echo "Copying results..."
scp -r -i $DIR/keys/cloudlab-sigmaos $USER@$LEADER_ADDR:~/ulambda/benchmarks/results/* $DIR/../benchmarks/results
echo "Done copying results..."

$DIR/stop-all.sh
