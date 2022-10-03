#!/bin/bash

# Generic make file

usage() {
  echo "Usage: $0 [--parallel]" 1>&2
}

PARALLEL=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="--parallel"
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

build_app="../generic-make.sh $PARALLEL"

# Build all the apps.
for d in `ls .`; do
  if [ -d $d ]; then
    cd $d
    if [ -z "$PARALLEL" ]; then
      eval "$build_app"
    else
      eval "$build_app" &
    fi
  fi
done
wait
