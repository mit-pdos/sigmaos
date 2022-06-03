#!/bin/bash

usage() {
  echo "Usage: $0 [--norace] [--vet] [--parallel] [--target <name>]" 1>&2
}

RACE="-race"
CMD="build"
TARGET="local"
PARALLEL=""
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
  *)
    echo "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

mkdir -p bin/kernel
mkdir -p bin/user
mkdir -p bin/realm

for k in `ls cmd`; do
  echo "Building $k components"
  for f in `ls cmd/$k`;  do
    if [ $CMD == "vet" ]; then
      echo "go vet cmd/$k/$f/main.go"
      go vet cmd/$k/$f/main.go
    else 
      echo "go build -ldflags='-X ulambda/ninep.Target=$TARGET' $RACE -o bin/$k/$f cmd/$k/$f/main.go"
      if [ -z "$PARALLEL" ]; then
        go build -ldflags="-X ulambda/ninep.Target=$TARGET" $RACE -o bin/$k/$f cmd/$k/$f/main.go
      else
        go build -ldflags="-X ulambda/ninep.Target=$TARGET" $RACE -o bin/$k/$f cmd/$k/$f/main.go &
      fi
    fi
  done
done

wait
