#!/bin/bash

DIR=$(dirname $0)

# Make results dirs
mkdir -p $DIR/benchmarks/results/burstbenchmark

GOGC=off ./bin/user/burstbenchmark $DIR/benchmarks/results
