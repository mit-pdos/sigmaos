#!/bin/bash

usage() {
  echo "Usage: $0 [--rustpath RUST] [--version VERSION] [-j NJOBS]" 1>&2
}

CARGO="cargo"
VERSION="1.0"
NJOBS=$(nproc)
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
  -j)
    shift
    NJOBS="$1"
    shift
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

TARGETS="uproc-trampoline spawn-latency hello-world-rs"

parallel --verbose --tag -j"$NJOBS" \
  --halt now,fail=1 \
  "$CARGO" build --manifest-path=rs/{1}/Cargo.toml --release \
  ::: $TARGETS

# Bail out early on build error
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

# Copy rust bins
cp rs/uproc-trampoline/target/release/uproc-trampoline bin/kernel
cp rs/spawn-latency/target/release/spawn-latency bin/user/spawn-latency-v$VERSION
cp rs/hello-world-rs/target/release/hello-world-rs bin/user/hello-world-rs-v$VERSION

# Build wasm scripts
TARGETS=$(ls rs/wasm)
parallel --verbose --tag -j"$NJOBS" \
  --halt now,fail=1 \
  "$CARGO" build --manifest-path=rs/wasm/{1}/Cargo.toml --target=wasm32-unknown-unknown --release \
  ::: $TARGETS

# Bail out early on build error
export EXIT_STATUS=$?
if [ $EXIT_STATUS  -ne 0 ]; then
  exit $EXIT_STATUS
fi

echo "Copy WASM scripts to bin dir"
for t in $TARGETS; do
  cp rs/wasm/$t/target/wasm32-unknown-unknown/release/$t.wasm $WASMDIR/
done
