#!/bin/bash

# optionally set SIGMADEBUG (see debug/flags.go) or SIGMAPERF (see
# perf/util.go). For example:

# export SIGMADEBUG="CONTAINER;KERNEL;"
# export SIGMAPERF="PROCD_PPROF;NAMED_CPU;"

# to find proxyd
export PATH=$PATH:$PWD/bin/linux/:$PWD/bin/kernel
ROOT=$(realpath $(dirname $0)/..)
if [ -f $ROOT/env/user.sh ]; then
  source $ROOT/env/user.sh
fi
