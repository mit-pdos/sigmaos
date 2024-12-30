#!/bin/bash

docker run --rm -t \
  --net=sigmaos-overlay-perf-test \
  arielszekely/sigmaos-netperf \
  go test -v netperf --run TestClntRTTTCP \
  --srvaddr $1 --ntrial 500
