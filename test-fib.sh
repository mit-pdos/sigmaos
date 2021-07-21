#!/bin/bash

usage() {
    echo "Usage: $0 [-naive]" 1>&2
}

NAIVE=0

while [[ "$#" -gt 0 ]]; do
    case "$1" in
    -naive)
        shift
        NAIVE=1
	;;
    *)
	error "unexpected argument $1"
	usage
	exit 1
	;;
    esac
done

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
rm -rf $fib_dir/.gg/*
GG_DIR=$fib_dir/.gg
./create-thunk.sh $N ./fib ./add

echo "4. Copying targets and thunks into memfs..."
cp -r ./.gg $memfs_dir
cp ./$target $memfs_dir

# Submit jobs to scheduler
echo "5. Running..."
if [[ $NAIVE -gt 0 ]]; then
  $ulambda_dir/mk-gg-ulambda-job-naive.sh $target | $ulambda_dir/bin/submit
else
  $ulambda_dir/mk-gg-ulambda-job.sh $target | $ulambda_dir/bin/submit
fi
