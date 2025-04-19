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

# Copy OpenBLAS-0.3.23
cp OpenBLAS-0.3.23/libopenblas64_p-r0.3.23.so $OUTPATH/kernel

# Inject custom Python lib
LIBDIR="cpython3.11/Lib"
cp ../sigmaos-local/pylib/splib.py $LIBDIR

# Add checksum overrides for default libraries
OVERRIDEFILE="sigmaos-checksum-override"
for entry in "$LIBDIR"/*; do
  if [ -e "$entry" ]; then
    if [ -d "$entry" ]; then
      touch "$entry/$OVERRIDEFILE"
    elif [[ -f "$entry" && "$entry" == *.py ]]; then
      filename=$(basename "$entry" .py)
      touch "$LIBDIR/$filename-$OVERRIDEFILE"
    fi
  fi
done

# Copy Python executable
cp cpython3.11/python $OUTPATH/kernel
cp -r cpython3.11 $OUTPATH/kernel
echo "/tmp/python/Lib" > $OUTPATH/kernel/python.pth # Dummy PYTHONPATH -- not used by actual program
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/kernel/pyvenv.cfg
cp cpython3.11/python $OUTPATH/user
cp -r cpython3.11 $OUTPATH/user
echo "/tmp/python/Lib" > $OUTPATH/user/python.pth
echo -e "home = /~~\ninclude-system-site-packages = false\nversion = 3.11.10" > $OUTPATH/user/pyvenv.cfg

# Copy and inject Python shim
gcc -Wall -fPIC -shared -o ld_fstatat.so ../sigmaos-local/ld_preload/ld_fstatat.c 
cp ld_fstatat.so $OUTPATH/kernel

# Build Python library
gcc -I../sigmaos-local -Wall -fPIC -shared -L/usr/lib -lprotobuf-c -o clntlib.so ../sigmaos-local/pylib/clntlib.c /usr/lib/libprotobuf-c.a ../sigmaos-local/pylib/proto/proc.pb-c.c ../sigmaos-local/pylib/proto/rpc.pb-c.c ../sigmaos-local/pylib/proto/sessp.pb-c.c ../sigmaos-local/pylib/proto/sigmap.pb-c.c ../sigmaos-local/pylib/proto/spproxy.pb-c.c ../sigmaos-local/pylib/proto/timestamp.pb-c.c
cp clntlib.so $OUTPATH/kernel

# Copy Python user processes
cp -r ../sigmaos-local/pyproc $OUTPATH/kernel

# Copy rust bins
cp ../sigmaos-local/rs/uproc-trampoline/target/release/uproc-trampoline $OUTPATH/kernel
cp ../sigmaos-local/rs/spawn-latency/target/release/spawn-latency $OUTPATH/user/spawn-latency-v$VERSION
