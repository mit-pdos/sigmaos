#!/bin/bash

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
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

TMP=/tmp/sigmaos

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP

# build and start db container
./start-db.sh

# build binaries for host
./make.sh --norace $PARALLEL linux

# build containers
DOCKER_BUILDKIT=1 docker build --build-arg parallel=$PARALLEL -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .
docker build -f Dockeruser -t sigmauser .
