#!/bin/bash

usage() {
  echo "Usage: $0 [--norace] [--vet] [--parallel] [--target TARGET] kernel|user" 1>&2
}

RACE="-race"
CMD="build"
TARGET="local"
PARALLEL=""
WHAT=""
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
  --target)
    shift
    TARGET="$1"
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
  kernel|user)
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

if [ $WHAT == "kernel" ]; then
    mkdir -p bin/kernel
    WHAT="kernel linux"
else
    mkdir -p bin/user
    WHAT="user"
fi

LDF="-X sigmaos/sigmap.Target=$TARGET"

for k in $WHAT; do
    echo "Building $k components"
    FILES=`ls cmd/$k`
    if [ $k == "user" ]; then
       FILES="sleeper exec-uproc"
    fi
    for f in $FILES;  do
        if [ $CMD == "vet" ]; then
            echo "go vet cmd/$k/$f/main.go"
            go vet cmd/$k/$f/main.go
        else
            GO="go"
            #      GO="~/go-custom/bin/go"
            build="$GO build -ldflags=\"$LDF\" $RACE -o bin/$k/$f cmd/$k/$f/main.go"
            echo $build
            if [ -z "$PARALLEL" ]; then
                eval "$build"
            else
                eval "$build" &
            fi
        fi
    done
done

wait

