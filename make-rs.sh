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

TARGETS="uproc-trampoline spawn-latency"

# If building in parallel, build with (n - 1) threads.
njobs=$(nproc)
njobs="$(($njobs-1))"
build="parallel -j$njobs $CARGO \"build --manifest-path=../sigmaos-local/rs/{}/Cargo.toml --release\" ::: $TARGETS"
echo $build
eval $build

# Copy Python executable
cp cpython3.11/python $OUTPATH/kernel
cp cpython3.11/pybuilddir.txt $OUTPATH/kernel
cp -r cpython3.11/Lib $OUTPATH/kernel
echo "/~~/Lib" > $OUTPATH/kernel/python._pth
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/kernel/pyvenv.cfg
cp cpython3.11/python $OUTPATH/user
cp cpython3.11/pybuilddir.txt $OUTPATH/user
cp -r cpython3.11/Lib $OUTPATH/user
echo "/~~/Lib" > $OUTPATH/user/python._pth
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/user/pyvenv.cfg

# Copy and inject Python libs
# cp ./pylib/splib.py Python-3.11.0/Lib
# cp Python-3.11.0/Lib $OUTPATH/kernel/pylib -r
# cp Python-3.11.0/Lib $OUTPATH/user -r
# cp Python-3.11.0/Lib $OUTPATH/user/pylib -r

# Copy and inject Python shim
gcc -Wall -fPIC -shared -o ld_fstatat.so ../sigmaos-local/ld_preload/ld_fstatat.c 
cp ld_fstatat.so $OUTPATH/kernel

# Copy Python user processes
cp -r pyproc $OUTPATH/kernel

# Copy rust bins
cp ../sigmaos-local/rs/uproc-trampoline/target/release/uproc-trampoline $OUTPATH/kernel
cp ../sigmaos-local/rs/spawn-latency/target/release/spawn-latency $OUTPATH/user/spawn-latency-v$VERSION
