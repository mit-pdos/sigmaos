#!/bin/bash

# Generic make file, used to build an app.

usage() {
  echo "Usage: $0 [--parallel] [--vet]" 1>&2
}

CMD="build"
PARALLEL=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="parallel"
    ;;
  --vet)
    shift
    CMD="vet"
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    error "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if ! [ -d cmd ]; then
  echo "Error: No cmd dir... did you run the script from within an app dir?"
  exit 1
fi

# Make the bin dir.
mkdir -p bin/

# Build all of the bins.
for f in `ls cmd`; do
  if [ $CMD == "vet" ]; then
  echo "go vet cmd/$f/main.go"
    go vet cmd/$f/main.go
  else 
    build="go build -o bin/$f cmd/$f/main.go"
    echo $build
    if [ -z "$PARALLEL" ]; then
      eval "$build"
    else
      eval "$build" &
    fi
  fi
done
wait
