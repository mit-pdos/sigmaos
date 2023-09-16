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

cleanup() {
  ./stop.sh --parallel --nopurge
  ./fsetcd-wipe.sh
}

go clean -testcache

if [[ $BASIC == "--basic" ]]; then

    #
    # test some support package
    #

    for T in path intervals serr linuxsched perf sigmap; do
        go test $VERB sigmaos/$T
        cleanup
    done

    #
    # test proxy with just named
    #

    go test $VERB sigmaos/proxy -start
    cleanup

    #
    # test with a kernel with just named
    #

    for T in semclnt electclnt; do
        go test $VERB -timeout 20m sigmaos/$T -start
        cleanup
    done

    # go test $VERB sigmaos/fslibsrv -start  # no perf

    # test memfs using schedd's memfs
    go test $VERB sigmaos/fslib -start -path "name/schedd/~local/" 
    cleanup
    go test $VERB sigmaos/memfs -start -path "name/schedd/~local/"
    cleanup

    #
    # tests a full kernel using root realm
    #

    for T in named procclnt ux s3 bootkernelclnt leaderclnt leadertest kvgrp sessclnt cachedsvcclnt www; do
        go test $VERB sigmaos/$T -start
        cleanup
    done

    go test $VERB sigmaos/fslibsrv -start -path "name/ux/~local/" -run ReadPerf
    cleanup
    go test $VERB sigmaos/fslibsrv -start -path "name/s3/~local/9ps3/" -run ReadPerf
    cleanup

    #
    # test with realms
    #

    go test $VERB sigmaos/realmclnt -start
    cleanup

fi

#
# app tests
#

if [[ $APPS == "--apps" ]]; then
    ./start-db.sh
    if [[ $FAST == "--fast" ]]; then
        go test $VERB sigmaos/mr -start -run "(MRJob|TaskAndCoord)"
        cleanup
        go test $VERB sigmaos/imgresized -start -run ImgdOne
        cleanup
        go test $VERB sigmaos/kv -start -run "(OKN|AllN)"
        cleanup
        go test $VERB sigmaos/hotel -start -run TestBenchDeathStarSingle
        cleanup
       	go test $VERB sigmaos/socialnetwork -start -run TestCompose
        cleanup
    else
        for T in imgresized mr kv hotel socialnetwork; do
            go test -timeout 20m $VERB sigmaos/$T -start
            cleanup
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
    cleanup
    go test $VERB sigmaos/cachedsvcclnt -start --overlays --run TestCacheClerk
    cleanup
    go test $VERB sigmaos/hotel -start --overlays --run GeoSingle
    cleanup
    go test $VERB sigmaos/hotel -start --overlays --run Www
    cleanup
    go test $VERB sigmaos/realmclnt -start --overlays --run Basic
    cleanup
    go test $VERB sigmaos/realmclnt -start --overlays --run WaitExitSimpleSingle
    cleanup
    go test $VERB sigmaos/realmclnt -start --overlays --run RealmNetIsolation
    cleanup
fi
