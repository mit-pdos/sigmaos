#!/bin/bash

DIR=$(dirname $0)
. $DIR/env/env.sh

export SIGMAROOTFS=$SIGMAROOTFS
PATH=$PATH:$PWD/bin/linux/

# default port number for named
export SIGMANAMED=":1111"

# optionally set SIGMADEBUG (see debug/flags.go) or SIGMAPERF (see
# perf/util.go). For example:

# export SIGMADEBUG="CONTAINER;KERNEL;"
# export SIGMAPERF="PROCD_PPROF;NAMED_CPU;"
