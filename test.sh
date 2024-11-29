#!/bin/bash

#
# Runs basic tests by default
# --apps: run app tests
# --apps-fast: run the fast app tests
#

usage() {
  echo "Usage: $0 [--apps-fast] [--apps] [--compile] [--usespproxyd] [--nodialproxy] [--reuse-kernel] [--cleanup] [--skipto PKG]" 
}

BASIC="--basic"
FAST=""
APPS=""
SPPROXYD=""
DIALPROXY=""
REUSEKERNEL=""
VERB="-v"
CONTAINER=""
SKIPTO=""
CLEANUP=""
COMPILE=""
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
        --skipto)
            shift
            SKIPTO="$1" 
            shift
            ;;
        --usespproxyd)
            shift
            SPPROXYD="--usespproxyd" 
            ;;
        --nodialproxy)
            shift
            DIALPROXY="--nodialproxy" 
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

    for T in path serr linuxsched util/perf sigmap dialproxy sessclnt npproxysrv fslib/reader fslib/writer stats fslib semclnt chunk/srv electclnt dircache memfs namesrv procclnt ux s3 bootkernelclnt leaderclnt leadertest apps/kv/kvgrp apps/cache/cachegrp/clnt apps/www sigmapsrv realm/clnt apps/mr apps/imgresize apps/kv apps/hotel apps/socialnetwork benchmarks benchmarks/remote example example_echo_server netperf ckpt; do
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

    for T in path serr linuxsched util/perf sigmap sortedmap; do
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

    for T in fslib/reader fslib/writer stats dialproxy fslib electclnt dircache; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB -timeout 20m sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL
        cleanup
    done

    # go test $VERB sigmaos/sigmapsrv -start  # no perf

    # test memfs
    go test $VERB sigmaos/fslib -start -path "name/memfs/~local/"  $SPPROXYD $DIALPROXY $REUSEKERNEL
    cleanup
    go test $VERB sigmaos/memfs -start $SPPROXYD $DIALPROXY $REUSEKERNEL
    cleanup

    #
    # tests a full kernel using root realm
    #

    for T in namesrv semclnt chunk/srv procclnt ux bootkernelclnt s3 leaderclnt leadertest apps/kv/kvgrp apps/cache/cachegrp/clnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL
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

    for T in realm/clnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
      go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL
      cleanup
  done
fi

#
# app tests
#

if [[ $APPS == "--apps" ]]; then
    if [[ $FAST == "--fast" ]]; then
        PKGS="apps/mr apps/imgresize apps/kv apps/hotel apps/socialnetwork"
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
          go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY -run "${TNAMES[$i]}"
          cleanup
          i=$(($i+1))
        done
#        go test $VERB sigmaos/apps/mr -start $SPPROXYD $DIALPROXY -run MRJob
#        cleanup
#        go test $VERB sigmaos/apps/imgresize -start $SPPROXYD $DIALPROXY -run ImgdOne
#        cleanup
#        go test $VERB sigmaos/apps/kv -start $SPPROXYD $DIALPROXY -run KVOKN
#        cleanup
#        ./start-db.sh
#        go test $VERB sigmaos/apps/hotel -start $SPPROXYD $DIALPROXY -run TestBenchDeathStarSingle
#        cleanup
#        ./start-db.sh
#       	go test $VERB sigmaos/apps/socialnetwork -start $SPPROXYD $DIALPROXY -run TestCompose
#        cleanup
    else
        for T in apps/imgresize apps/mr apps/hotel apps/socialnetwork apps/www; do
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
            go test -timeout 20m $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL
            cleanup
        done
        # On machines with many cores, kv tests may take a long time.
        for T in apps/kv; do
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
            go test -timeout 50m $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL
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
