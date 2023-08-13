#!/bin/bash

#
# Runs all tests by default.
# --apps: run only app tests
# --fast: run only the key tests
#

usage() {
  echo "Usage: $0 [--fast] [--apps]" 
}

FAST=""
APPS=""
VERB="-v"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --fast)
    shift
    FAST="--fast"
    ;;
  --apps)
    shift
    APPS="--apps"
    ;;
   *)
   echo "unexpected argument $1"
   usage
   exit 1
 esac
done

go clean -testcache

if [[ $APPS == "" ]]; then

    #
    # test some support package
    #

    for T in path intervals serr linuxsched perf sigmap; do
        go test $VERB sigmaos/$T
    done

    #
    # test proxy with just named
    #

    go test $VERB sigmaos/proxy -start

    #
    # test with a kernel with just named
    #

    for T in reader writer stats fslib semclnt electclnt; do
        go test $VERB sigmaos/$T -start
    done

    # go test $VERB sigmaos/memfs -start   # no pipes
    # go test $VERB sigmaos/fslibsrv -start  # no perf

    # test memfs using schedd's memfs
    go test $VERB sigmaos/fslib -start -path "name/schedd/~local/" 
    go test $VERB sigmaos/memfs -start -path "name/schedd/~local/"

    #
    # tests a full kernel using root realm
    #

    for T in named procclnt ux s3 bootkernelclnt leaderclnt leadertest kvgrp sessclnt cachedsvcclnt www; do
        go test $VERB sigmaos/$T -start
    done


    go test $VERB sigmaos/fslibsrv -start -path "name/ux/~local/" -run ReadPerf
    go test $VERB sigmaos/fslibsrv -start -path "name/s3/~local/9ps3/" -run ReadPerf
fi

#
# apps tests
#

if [[ $FAST == "" ]]; then
    for T in imgresized mr kv hotel; do
        go test $VERB sigmaos/$T -start
    done
else
    go test $VERB sigmaos/mr -start -run "(MRJob|TaskAndCoord)"
    go test $VERB sigmaos/imgresized -start -run ImgdOne
    go test $VERB sigmaos/kv -start -run "(OKN|AllN)"
    go test $VERB sigmaos/hotel -start -run TestBenchDeathStarSingle
fi

if [[ $APPS == "" ]]; then

    #
    # test with realms
    #

    go test $VERB sigmaos/realmclnt -start

    #
    # Container tests (will OOM your machine if you don't have 1:1 memory:swap ratio
    #
    go test $VERB sigmaos/container -start

    #
    # tests with overlays
    #

    go test $VERB sigmaos/procclnt -start --overlays --run TestWaitExitSimpleSingle
    go test $VERB sigmaos/cachedsvcclnt -start --overlays --run TestCacheClerk
    go test $VERB sigmaos/hotel -start --overlays --run GeoSingle
    go test $VERB sigmaos/hotel -start --overlays --run Www
    go test $VERB sigmaos/realmclnt -start --overlays --run Basic
    go test $VERB sigmaos/realmclnt -start --overlays --run WaitExitSimpleSingle
    go test $VERB sigmaos/realmclnt -start --overlays --run RealmNetIsolation
fi
