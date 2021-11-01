#!/bin/bash

DIR=$(dirname $0)

for n in 3 2 1 
do
  export N_REPLICAS=$n
  $DIR/run-microbenchmarks.sh
done
