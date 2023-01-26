#!/bin/bash

TMP=/tmp/sigmaos

# boot and tests uses hosts /tmp, which mounted in kernel container.
mkdir -p $TMP

# copy boot ymls, which be filled out in more detail during various stages
cp bootparam/*.yml $TMP/

# build and start db container
./build-db.sh $TMP/bootall.yml $TMP/bootmach.yml

# build binaries for host
./make.sh --norace linux

# build containers
DOCKER_BUILDKIT=1 docker build -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .
docker build -f Dockeruser -t sigmauser .
