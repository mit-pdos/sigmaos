#!/bin/bash

memfs_dir=/mnt/9p/fs/gg

if [ ! -d "/mnt/9p/fs" ]
then
  echo "9p not mounted!"
  exit 1
fi

echo "1. Set up memfs dirs..."
mkdir -p $memfs_dir
mkdir -p $memfs_dir/results

# fib(N) value to compute
N=10
target=fib${N}_output

# Set up env vars
cd $HOME/gg
. ./.environment

fib_dir=$HOME/gg/examples/fibonacci
ulambda_dir=$HOME/ulambda

# Remove output from prior runs
rm $fib_dir/$target

# Remove old local environments
rm -rf /tmp/ulambda/*

# Remove cache
rm -rf $HOME/.cache/gg

cd $fib_dir

echo "2. Init gg..."
gg init

# Create initial fib thunk(s) to run
echo "3. Generating input thunks..."
GG_DIR=$fib_dir/.gg
./create-thunk.sh $N ./fib ./add

echo "4. Copying targets and thunks into memfs..."
cp -r ./.gg $memfs_dir
cp ./$target $memfs_dir

# Submit jobs to scheduler
echo "5. Running..."
$ulambda_dir/mk-gg-ulambda-job.sh $target | $ulambda_dir/bin/submit
