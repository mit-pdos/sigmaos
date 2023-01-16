#!/bin/bash

DIR=$(dirname $0)
. $DIR/env/env.sh

export SIGMAROOTFS=$SIGMAROOTFS
PATH=$PATH:$PWD/bin/linux/

# optionally set SIGMADEBUG (see debug/flags.go) or SIGMAPERF (see
# perf/util.go). For example:

# export SIGMADEBUG="CONTAINER;KERNEL;"
# export SIGMAPERF="PROCD_PPROF;NAMED_CPU;"
