#!/bin/bash

#
# Runs basic tests by default
# --apps: run app tests
# --apps-fast: run the fast app tests
# --overlay: run overlay tests
#

usage() {
  echo "Usage: $0 [--apps-fast] [--apps] [--compile] [--overlay HOST_IP] [--gvisor] [--usespproxyd] [--nonetproxy] [--reuse-kernel] [--cleanup] [--skipto PKG]" 
}

BASIC="--basic"
FAST=""
APPS=""
OVERLAY=""
GVISOR=""
SPPROXYD=""
NETPROXY=""
REUSEKERNEL=""
VERB="-v"
CONTAINER=""
SKIPTO=""
CLEANUP=""
COMPILE=""
HOST_IP="IP_NOT_SET"
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
        --compile)
            shift
            BASIC=""
            COMPILE="--compile"
            ;;
        --overlay)
            shift
            BASIC="" 
            OVERLAY="--overlay"
            HOST_IP="$1"
            shift
            ;;
        --skipto)
            shift
            SKIPTO="$1" 
            shift
            ;;
        --gvisor)
            shift
            GVISOR="--gvisor" 
            ;;
        --usespproxyd)
            shift
            SPPROXYD="--usespproxyd" 
            ;;
        --nonetproxy)
            shift
            NETPROXY="--nonetproxy" 
            ;;
        --reuse-kernel)
            shift
            REUSEKERNEL="--reuse-kernel"
            ;;
        --cleanup)
            shift
            CLEANUP="true" 
            ;;
        *)
            echo "unexpected argument $1"
            usage
            exit 1
    esac
done

cleanup() {
  if [[ "$CLEANUP" == "true" ]]; then
    ./stop.sh --parallel --nopurge
    ./fsetcd-wipe.sh
  fi
}

go clean -testcache
cleanup

if ! [ -z "$REUSEKERNEL" ]; then
  if [ -z "$CLEANUP" ]; then
    echo "Must use flag --cleanup when using flag --reuse-kernel"
    exit 1
  fi
fi

if [[ $COMPILE == "--compile" ]]; then

    #
    # test if test packages compile
    #

    for T in path intervals serr linuxsched perf sigmap netproxy sessclnt npproxysrv reader writer stats fslib semclnt chunksrv electclnt dircache memfs namesrv procclnt ux s3 bootkernelclnt leaderclnt leadertest kvgrp cachedsvcclnt www sigmapsrv realmclnt mr imgresizesrv kv hotel socialnetwork benchmarks benchmarks/remote example example_echo_server netperf; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T --run TestCompile
    done
fi

if [[ $BASIC == "--basic" ]]; then

    #
    # test some support package
    #

    for T in path intervals serr linuxsched perf sigmap sortedmap; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T
        cleanup
    done

    #
    # test sessions
    #
    
    for T in sessclnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T
        cleanup
    done

    #
    # test with a kernel with just named
    #

    for T in reader writer stats netproxy fslib electclnt dircache; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB -timeout 20m sigmaos/$T -start $SPPROXYD $NETPROXY $REUSEKERNEL
        cleanup
    done

    # go test $VERB sigmaos/sigmapsrv -start  # no perf

    # test memfs
    go test $VERB sigmaos/fslib -start -path "name/memfs/~local/"  $SPPROXYD $NETPROXY $REUSEKERNEL
    cleanup
    go test $VERB sigmaos/memfs -start $SPPROXYD $NETPROXY $REUSEKERNEL
    cleanup

    #
    # tests a full kernel using root realm
    #

    for T in namesrv semclnt chunksrv procclnt ux bootkernelclnt s3 leaderclnt leadertest kvgrp cachedsvcclnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T -start $GVISOR  $SPPROXYD $NETPROXY $REUSEKERNEL
        cleanup
    done

    #
    # test npproxy with just named and full kernel
    #

    for T in npproxysrv; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T -start
        cleanup
    done


    go test $VERB sigmaos/sigmapsrv -start -path "name/ux/~local/" -run ReadPerf
    cleanup
    go test $VERB sigmaos/sigmapsrv -start -path "name/s3/~local/9ps3/" -run ReadPerf
    cleanup
    go test $VERB sigmaos/sigmapsrv --withs3pathclnt -start -path "name/s3/~local/9ps3/" -run ReadFilePerfSingle
    cleanup
    

    #
    # test with realms
    #

    for T in realmclnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
      go test $VERB sigmaos/$T -start $GVISOR $SPPROXYD $NETPROXY $REUSEKERNEL
      cleanup
  done
fi

#
# app tests
#

if [[ $APPS == "--apps" ]]; then
    if [[ $FAST == "--fast" ]]; then
        PKGS="mr imgresizesrv kv hotel socialnetwork"
        TNAMES=("MRJob" "ImgdOne" "KVOKN" "TestBenchDeathStarSingle" "TestCompose")
        NEED_DB=("false" "false" "false" "true" "true")
        i=0
        for T in $PKGS; do
          if ! [ -z "$SKIPTO" ]; then
            if [[ "$SKIPTO" == "$T" ]]; then
              # Stop skipping
              SKIPTO=""
            else
              # Skip
              continue
            fi
          fi
          if [[ "${NEED_DB[$i]}" == "true" ]]; then
            ./start-db.sh
          fi
          go test $VERB sigmaos/$T -start $GVISOR $SPPROXYD $NETPROXY -run "${TNAMES[$i]}"
          cleanup
          i=$(($i+1))
        done
#        go test $VERB sigmaos/mr -start $GVISOR $SPPROXYD $NETPROXY -run MRJob
#        cleanup
#        go test $VERB sigmaos/imgresizesrv -start $GVISOR $SPPROXYD $NETPROXY -run ImgdOne
#        cleanup
#        go test $VERB sigmaos/kv -start $GVISOR $SPPROXYD $NETPROXY -run KVOKN
#        cleanup
#        ./start-db.sh
#        go test $VERB sigmaos/hotel -start $GVISOR $SPPROXYD $NETPROXY -run TestBenchDeathStarSingle
#        cleanup
#        ./start-db.sh
#       	go test $VERB sigmaos/socialnetwork -start $GVISOR $SPPROXYD $NETPROXY -run TestCompose
#        cleanup
    else
        for T in imgresizesrv mr hotel socialnetwork www; do
            if ! [ -z "$SKIPTO" ]; then
              if [[ "$SKIPTO" == "$T" ]]; then
                # Stop skipping
                SKIPTO=""
              else
                # Skip
                continue
              fi
            fi
            ./start-db.sh
            go test -timeout 20m $VERB sigmaos/$T -start $GVISOR $SPPROXYD $NETPROXY $REUSEKERNEL
            cleanup
        done
        # On machines with many cores, kv tests may take a long time.
        for T in kv; do
            if ! [ -z "$SKIPTO" ]; then
              if [[ "$SKIPTO" == "$T" ]]; then
                # Stop skipping
                SKIPTO=""
              else
                # Skip
                continue
              fi
            fi
            ./start-db.sh
            go test -timeout 50m $VERB sigmaos/$T -start $GVISOR $SPPROXYD $NETPROXY $REUSEKERNEL
            cleanup
        done
    fi
fi

#
# Container tests (will OOM your machine if you don't have 1:1 memory:swap ratio)
#

if [[ $CONTAINER == "--container" ]] ; then
    go test $VERB sigmaos/scontainer -start
fi

#
# Overlay network tests
#

if [[ $OVERLAY == "--overlay" ]] ; then
    if [ "$HOST_IP" == "IP_NOT_SET" ] || [ -z "$HOST_IP" ]; then
      echo "ERROR: Host IP not provided"
      exit 1
    fi
    echo "Overlay tests running with host IP $HOST_IP"
    ./start-network.sh
    
    go test $VERB sigmaos/procclnt --etcdIP $HOST_IP -start $GVISOR --overlays --run TestWaitExitSimpleSingle
    cleanup
    go test $VERB sigmaos/cachedsvcclnt --etcdIP $HOST_IP -start $GVISOR --overlays --run TestCacheClerk
    cleanup
    ./start-db.sh
    go test $VERB sigmaos/hotel --etcdIP $HOST_IP -start $GVISOR --overlays --run GeoSingle
    cleanup
    ./start-db.sh
    go test $VERB sigmaos/hotel --etcdIP $HOST_IP -start $GVISOR --overlays --run Www
    cleanup
    go test $VERB sigmaos/realmclnt --etcdIP $HOST_IP -start $GVISOR --overlays --run Basic
    cleanup
    go test $VERB sigmaos/realmclnt --etcdIP $HOST_IP -start $GVISOR --overlays --run WaitExitSimpleSingle
    cleanup
    go test $VERB sigmaos/realmclnt --etcdIP $HOST_IP -start $GVISOR --overlays --run RealmNetIsolation
    cleanup
fi
