#!/bin/bash

usage() {
  echo "Usage: $0 [--norace] [--vet] [--parallel] [--gopath GO] [--target TARGET] [--userbin USERBIN] kernel|user|proxy" 1>&2
}

RACE="-race"
CMD="build"
TARGET="local"
USERBIN="all"
GO="go"
PARALLEL=""
WHAT=""
BINS=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --norace)
    shift
    RACE=""
    ;;
  --vet)
    shift
    CMD="vet"
    ;;
  --gopath)
    shift
    GO="$1"
    shift
    ;;
  --target)
    shift
    TARGET="$1"
    shift
    ;;
  --userbin)
    shift
    USERBIN="$1"
    shift
    ;;
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --bins)
    shift
    BINS="$1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  kernel|user|linux|proxy)
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
echo $WHAT

OUTPATH=bin

if [[ $WHAT == "kernel" ]]; then
    mkdir -p $OUTPATH/kernel
    mkdir -p $OUTPATH/linux
    WHAT="kernel linux"
elif [[ $WHAT == "user" ]]; then
    mkdir -p $OUTPATH/user
elif [[ $WHAT == "proxy" ]]; then
    mkdir -p $OUTPATH/proxy
else
    mkdir -p $OUTPATH/linux
    WHAT="linux"
fi

LDF="-X sigmaos/sigmap.Target=$TARGET -s -w"

for k in $WHAT; do
  echo "Building $k components"
  FILES=`ls cmd/$k`
   if [[ "$k" == "user" ]] && ! [[ "$USERBIN" == "all" ]] ; then
     FILES="$(echo "$USERBIN" | tr "," " ")"
     echo "Only building userbin $USERBIN files $FILES"
   fi
  if [ -z "$PARALLEL" ]; then
    for f in $FILES;  do
      if [ $CMD == "vet" ]; then
        echo "$GO vet cmd/$k/$f/main.go"
        $GO vet cmd/$k/$f/main.go
      else
        build="$GO build -ldflags=\"$LDF\" $RACE -o $OUTPATH/$k/$f cmd/$k/$f/main.go"
        echo $build
        eval "$build"
      fi
    done
  else
    # If building in parallel, build with (n - 1) threads.
    njobs=$(nproc)
    njobs="$(($njobs-1))"
    build="parallel -j$njobs $GO \"build -ldflags='$LDF' $RACE -o $OUTPATH/$k/{} cmd/$k/{}/main.go\" ::: $FILES"
    echo $build
    eval $build
  fi
done
