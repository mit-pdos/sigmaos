#!/bin/bash

# fib(N) value to compute
N=10

# Set up env vars
cd $HOME/gg
. ./.environment

fib_dir=$HOME/gg/examples/fibonacci
ulambda_dir=$HOME/ulambda

# Remove output from prior runs
rm $fib_dir/fib${N}_output

# Remove old local environments
rm -rf /tmp/ulambda/*

# Remove cache
rm -rf $HOME/.cache/gg

cd $fib_dir

echo "1. Init gg..."
gg init

# Create initial fib thunk(s) to run
echo "2. Generating input thunks..."
GG_DIR=$fib_dir/.gg
./create-thunk.sh $N ./fib ./add

# Submit jobs to schedd
echo 'Running...'
$ulambda_dir/mk-gg-ulambda-job.sh fib${N}_output | $ulambda_dir/bin/submit
