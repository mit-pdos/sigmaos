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

mkdir -p bin/

for f in `ls cmd`
do
    if [ $CMD == "vet" ]; then
  echo "go vet cmd/$f/main.go"
  go vet cmd/$f/main.go
    else 
  echo "go build $RACE -o bin/$f cmd/$f/main.go"
  go build $RACE -o bin/$f cmd/$f/main.go
    fi
done
