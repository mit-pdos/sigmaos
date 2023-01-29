#!/bin/bash

TMP=/tmp/sigmaos

# tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP

# build and start db container
# ./build-db.sh $TMP/bootall.yml $TMP/bootmach.yml

# build binaries for host
./make.sh --norace linux

# build containers
DOCKER_BUILDKIT=1 docker build -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .
docker build -f Dockeruser -t sigmauser .
