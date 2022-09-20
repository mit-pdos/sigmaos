#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

DIR=$(dirname $0)

. $DIR/config

mkdir -p $DIR/../benchmarks/results/logs

scp -i $DIR/keys/cloudlab-sigmaos $1:~/sigmaos/machined.out $DIR/../benchmarks/results/logs/$1_${N_REPLICAS}_replicas.out
