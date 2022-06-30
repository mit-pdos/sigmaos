#!/bin/bash

DIR=$(dirname $0)

# Make results dirs
mkdir -p $DIR/benchmarks/results/microbenchmarks
mkdir -p $DIR/benchmarks/results/pprof

GOGC=off ./bin/user/microbenchmarks $DIR/benchmarks/results
