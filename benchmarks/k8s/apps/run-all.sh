#!/bin/bash

usage() {
  echo "Usage: $0 --script SCRIPT [--parallel]" 1>&2
}

SCRIPT=""
PARALLEL=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --script)
    shift
    SCRIPT=$1
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
    error "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$SCRIPT" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

cmd="../$SCRIPT $PARALLEL"

# Build all the apps.
for d in `ls .`; do
  if [ -d $d ]; then
    cd $d
    if [ -z "$PARALLEL" ]; then
      eval "$cmd"
    else
      eval "$cmd" &
    fi
    cd ..
  fi
done
wait
