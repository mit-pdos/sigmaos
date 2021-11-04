#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

mkdir -p $DIR/../benchmarks/results/logs

scp -i $DIR/keys/cloudlab-sigmaos $1:~/ulambda/realmd.out $DIR/../benchmarks/results/logs/$1_${N_REPLICAS}_replicas.out
