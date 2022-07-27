#!/bin/bash

usage() {
  echo "Usage: $0 [--norace] [--vet] [--parallel] [--target TARGET] [--version VERSION]" 1>&2
}

RACE="-race"
CMD="build"
TARGET="local"
VERSION=$(date +%s)
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
  --version)
    shift
    VERSION=$1
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/.env

echo $VERSION > $VERSION_FILE
echo $VERSION_FILE

mkdir -p bin/kernel
mkdir -p bin/user
mkdir -p bin/realm

LDF="-X ulambda/ninep.Target=$TARGET -X ulambda/proc.Version=$VERSION"

for k in `ls cmd`; do
  echo "Building $k components"
  for f in `ls cmd/$k`;  do
    if [ $CMD == "vet" ]; then
      echo "go vet cmd/$k/$f/main.go"
      go vet cmd/$k/$f/main.go
    else 
      build="go build -ldflags=\"$LDF\" $RACE -o bin/$k/$f cmd/$k/$f/main.go"
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

#LDF="-X 'ulambda/ninep.Target=$TARGET' -X 'ulambda/ninep.Version=$VERSION'"
#
#for k in `ls cmd`; do
#  echo "Building $k components"
#  for f in `ls cmd/$k`;  do
#    if [ $CMD == "vet" ]; then
#      echo "go vet cmd/$k/$f/main.go"
#      go vet cmd/$k/$f/main.go
#    else 
#      echo "go build -ldflags=\"$LDF\" $RACE -o bin/$k/$f cmd/$k/$f/main.go"
#      if [ -z "$PARALLEL" ]; then
#        go build -ldflags="$LDF" $RACE -o bin/$k/$f cmd/$k/$f/main.go
#      else
#        go build -ldflags="$LDF" $RACE -o bin/$k/$f cmd/$k/$f/main.go &
#      fi
#    fi
#  done
#done
#
#w
