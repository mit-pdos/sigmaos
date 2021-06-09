#!/bin/sh

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

for f in `ls cmd`
do
    echo "go build $RACE -o bin/$f cmd/$f/main.go"
    go build $RACE -o bin/$f cmd/$f/main.go
done

echo "Build c_spinner"
cd perf/c-spinner
make
