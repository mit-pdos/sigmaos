#!/bin/bash

usage() {
  echo "Usage: $0 --tag TAG [--target target ] [--parallel]" 1>&2
}

PARALLEL=""
TAG=""
TARGET="local"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --tag)
    shift
    TAG="$1"
    shift
    ;;
  --target)
    shift
    TARGET="$1"
    shift
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

if [ -z "$TAG" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi


TMP=/tmp/sigmaos

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP

# build and start db container
if [ "${TARGET}" != "aws" ]; then
   ./start-db.sh
fi

# XXX this will do go somewhere else
if ! docker node ls | grep -q 'Leader'; then
    docker swarm init
fi 

# XXX move and one per realm
if ! docker network ls | grep -q 'sigmanet'; then
    docker network create --driver overlay sigmanet --attachable
fi
   

# build binaries for host
./make.sh --norace $PARALLEL linux

# build containers
DOCKER_BUILDKIT=1 docker build --build-arg target=$TARGET --build-arg parallel=$PARALLEL -t arielszekely/sigmabase .
docker tag arielszekely/sigmabase arielszekely/sigmabase:$TAG
docker push arielszekely/sigmabase:$TAG
docker build -f Dockerkernel -t arielszekely/sigmaos .
docker tag arielszekely/sigmaos arielszekely/sigmaos:$TAG
docker push arielszekely/sigmaos:$TAG
docker build -f Dockeruser -t arielszekely/sigmauser .
docker tag arielszekely/sigmauser arielszekely/sigmauser:$TAG
docker push arielszekely/sigmauser:$TAG
