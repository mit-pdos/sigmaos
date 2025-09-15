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
OUTPATH=bin
WASMDIR=$OUTPATH/wasm

mkdir -p $OUTPATH/kernel
mkdir -p $OUTPATH/user
mkdir -p $WASMDIR

LDF="-X sigmaos/sigmap.Target=$TARGET -s -w"

TARGETS="uproc-trampoline spawn-latency"

# If building in parallel, build with (n - 1) threads.
njobs=$(nproc)
njobs="$(($njobs-1))"
build="parallel -j$njobs $CARGO \"build --manifest-path=rs/{}/Cargo.toml --release\" ::: $TARGETS"
echo $build
eval $build

# Bail out early on build error
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy rust bins
cp rs/uproc-trampoline/target/release/uproc-trampoline bin/kernel
cp rs/spawn-latency/target/release/spawn-latency bin/user/spawn-latency-v$VERSION

# Build wasm scripts
TARGETS=$(ls rs/wasm)
build="parallel -j$njobs $CARGO \"build --manifest-path=rs/wasm/{}/Cargo.toml --target=wasm32-unknown-unknown --release\" ::: $TARGETS"
echo $build
eval $build

# Bail out early on build error
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

echo "Copy WASM scripts to bin dir"
for t in $TARGETS; do
  cp rs/wasm/$t/target/wasm32-unknown-unknown/release/$t.wasm $WASMDIR/
done
