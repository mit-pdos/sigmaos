#!/bin/bash

# optionally set SIGMADEBUG (see debug/flags.go) or SIGMAPERF (see
# perf/util.go). For example:

export SIGMADEBUG="ECHO_SERVER;"

# export SIGMADEBUG="CONTAINER;KERNEL;"
# export SIGMAPERF="PROCD_PPROF;NAMED_CPU;"

# to find proxyd
export PATH=$PATH:$PWD/bin/linux/
