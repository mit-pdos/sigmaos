#!/bin/bash

usage() {
  echo "Usage: $0 [--parallel]" 1>&2
}

PARALLEL=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  -help)
    usage
    exit 0
    ;;
  kernel|user|linux)
    WHAT=$1
    shift
    ;;
  *)
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

ROOT=$(pwd)
OUTPATH=bin

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/linux

LDF="-X sigmaos/sigmap.Target=$TARGET -s -w"

TARGETS="exec-uproc-rs spawn-latency"

# If building in parallel, build with (n - 1) threads.
njobs=$(nproc)
njobs="$(($njobs-1))"
build="parallel -j$njobs bash \"-c cd rs/{} && \$HOME/.cargo/bin/cargo build --release\" ::: $TARGETS"
echo $build
eval $build
