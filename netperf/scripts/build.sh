#!/bin/bash

DIR=$(realpath $(dirname $0))

cd $DIR/..
docker build -t arielszekely/sigmaos-netperf -f docker/Dockerfile . && \
  docker push arielszekely/sigmaos-netperf
