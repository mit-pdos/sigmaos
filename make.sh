#!/bin/bash

RACE="-race"

usage() {
    echo "Usage: $0 [-norace] [-vet]" 1>&2
}

CMD="build"

while [[ "$#" -gt 0 ]]; do
    case "$1" in
    -norace)
        shift
        RACE=""
	;;
    -vet)
	shift
	CMD="vet"
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
      if [ $CMD == "vet" ]; then
	  echo "go vet cmd/$k/$f/main.go"
	  go vet cmd/$k/$f/main.go
      else 
	  echo "go build $RACE -o bin/$k/$f cmd/$k/$f/main.go"
	  go build $RACE -o bin/$k/$f cmd/$k/$f/main.go
      fi
  done
done

echo "Build c_spinner"
cd benchmarks/c-spinner
make
