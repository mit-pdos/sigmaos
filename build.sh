#!/bin/bash

# some tests produce output in tmp/
mkdir -p /tmp/sigmaos

# build binaries for host
./make.sh --norace linux

# build containers
DOCKER_BUILDKIT=1 docker build -t sigmaosbase .
docker build -f Dockerkernel -t sigmaos .
docker build -f Dockeruser -t sigmauser .
