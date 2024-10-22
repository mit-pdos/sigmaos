#!/bin/bash

usage() {
  echo "Usage: $0 [--rustpath RUST] [--version VERSION] [--parallel]" 1>&2
}

CARGO="cargo"
VERSION="1.0"
PARALLEL=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --rustpath)
    shift
    CARGO="$1"
    shift
    ;;
  --version)
    shift
    VERSION="$1"
    shift
    ;;
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
OUTPATH=../sigmaos-local/bin

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/user

LDF="-X sigmaos/sigmap.Target=$TARGET -s -w"

TARGETS="exec-uproc-rs spawn-latency"

# If building in parallel, build with (n - 1) threads.
njobs=$(nproc)
njobs="$(($njobs-1))"
build="parallel -j$njobs $CARGO \"build --manifest-path=../sigmaos-local/rs/{}/Cargo.toml --release\" ::: $TARGETS"
echo $build
eval $build

# Copy Python executable
cp Python-3.11.0/python $OUTPATH/kernel
cp Python-3.11.0/python $OUTPATH/user

# Copy pybuilddir.txt
cp Python-3.11.0/pybuilddir.txt $OUTPATH/kernel
cp Python-3.11.0/pybuilddir.txt $OUTPATH/user

# Copy and inject Python libs
cp ../sigmaos-local/pylib/splib.py Python-3.11.0/Lib
cp Python-3.11.0/Lib $OUTPATH/kernel/pylib -r

# Copy and inject Python shim
gcc -Wall -fPIC -shared -o ld_fstatat.so ../sigmaos-local/ld_preload/ld_fstatat.c 
cp ld_fstatat.so $OUTPATH/kernel

# Copy rust bins
cp ../sigmaos-local/rs/exec-uproc-rs/target/release/exec-uproc-rs $OUTPATH/kernel
cp ../sigmaos-local/rs/spawn-latency/target/release/spawn-latency $OUTPATH/user/spawn-latency-v$VERSION
