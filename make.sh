#!/bin/bash

RACE="-race"

usage() {
    echo "Usage: $0 [-norace]" 1>&2
}

while [[ "$#" -gt 0 ]]; do
    case "$1" in
    -norace)
        shift
        RACE=""
	;;
    *)
	error "unexpected argument $1"
	usage
	exit 1
	;;
    esac
done

mkdir -p bin/kernel
mkdir -p bin/user

for k in `ls cmd`
do
  echo "Building $k components"
  for f in `ls cmd/$k`
  do
      echo "go build $RACE -o bin/$k/$f cmd/$k/$f/main.go"
      go build $RACE -o bin/$k/$f cmd/$k/$f/main.go
  done
done

echo "Build c_spinner"
cd perf/c-spinner
make
