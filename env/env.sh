#!/bin/bash

# optionally set SIGMADEBUG (see debug/flags.go) or SIGMAPERF (see
# perf/util.go). For example:

# export SIGMADEBUG="CONTAINER;KERNEL;"
# export SIGMAPERF="PROCD_PPROF;NAMED_CPU;"

# to find proxyd
export PATH=$PATH:$PWD/bin/linux/:$PWD/bin/kernel
# export SIGMAUSER="NOT_SET" # uncomment and change to your username to enable development on shared machines
