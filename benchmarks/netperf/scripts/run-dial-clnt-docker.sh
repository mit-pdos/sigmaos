#!/bin/bash

docker run --rm -t \
  --net=sigmaos-overlay-perf-test \
  arielszekely/sigmaos-netperf \
  go test -v netperf --run TestClntDialTCP \
  --srvaddr $1 --ntrial 50
