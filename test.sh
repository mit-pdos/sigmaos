#!/bin/bash

#
# Runs basic tests by default
# --apps: run app tests
# --apps-fast: run the fast app tests
# --overlay: run overlay tests
#

usage() {
  echo "Usage: $0 [--apps-fast] [--apps] [--overlay]" 
}

BASIC="--basic"
FAST=""
APPS=""
OVERLAY=""
VERB="-v"
CONTAINER=""
while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --apps-fast)
            shift
            BASIC="" 
            APPS="--apps"
            FAST="--fast"
            ;;
        --apps)
            shift
            BASIC="" 
            APPS="--apps"
            ;;
        --overlay)
            shift
            BASIC="" 
            OVERLAY="--overlay"
            ;;
        *)
            echo "unexpected argument $1"
            usage
            exit 1
    esac
done

go clean -testcache

if [[ $BASIC == "--basic" ]]; then

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

    #
    # test with realms
    #

    go test $VERB sigmaos/realmclnt -start

fi

#
# app tests
#

if [[ $APPS == "--apps" ]]; then
    ./start-db.sh
    if [[ $FAST == "--fast" ]]; then
        go test $VERB sigmaos/mr -start -run "(MRJob|TaskAndCoord)"
        go test $VERB sigmaos/imgresized -start -run ImgdOne
        go test $VERB sigmaos/kv -start -run "(OKN|AllN)"
        go test $VERB sigmaos/hotel -start -run TestBenchDeathStarSingle
		go test $VERB sigmaos/socialnetwork -start -run TestCompose
    else
        for T in imgresized mr kv hotel socialnetwork; do
            go test -timeout 20m $VERB sigmaos/$T -start
        done
    fi
fi

#
# Container tests (will OOM your machine if you don't have 1:1 memory:swap ratio)
#

if [[ $CONTAINER == "--container" ]] ; then
    go test $VERB sigmaos/container -start
fi

#
# Overlay network tests
#

if [[ $OVERLAY == "--overlay" ]] ; then
    ./start-db.sh
    ./start-network.sh
    
    go test $VERB sigmaos/procclnt -start --overlays --run TestWaitExitSimpleSingle
    go test $VERB sigmaos/cachedsvcclnt -start --overlays --run TestCacheClerk
    go test $VERB sigmaos/hotel -start --overlays --run GeoSingle
    go test $VERB sigmaos/hotel -start --overlays --run Www
    go test $VERB sigmaos/realmclnt -start --overlays --run Basic
    go test $VERB sigmaos/realmclnt -start --overlays --run WaitExitSimpleSingle
    go test $VERB sigmaos/realmclnt -start --overlays --run RealmNetIsolation
fi
