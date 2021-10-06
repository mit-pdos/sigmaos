#!/bin/bash

DIR=$(dirname $0)

./start.sh

# Make results dirs
mkdir -p $DIR/benchmarks/results/microbenchmarks
mkdir -p $DIR/benchmarks/results/pprof

GOGC=off ./bin/user/microbenchmarks $DIR/benchmarks/results

./stop.sh
