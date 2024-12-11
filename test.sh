#!/bin/bash

#
# Runs basic tests by default
# --apps: run app tests
# --apps-fast: run the fast app tests
#

usage() {
  echo "Usage: $0 [--apps-fast] [--apps] [--compile] [--usespproxyd] [--nodialproxy] [--reuse-kernel] [--cleanup] [--skipto PKG] [--savelogs]" 
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
SAVELOGS=""
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
        --savelogs)
            shift
            SAVELOGS="true" 
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

LOG_DIR="/tmp/sigmaos-test-logs"
rm -rf $LOG_DIR
mkdir $LOG_DIR

cleanup() {
  if [[ "$CLEANUP" == "true" ]]; then
    ./stop.sh --parallel --nopurge
    ./fsetcd-wipe.sh
  fi
}

# Save test logs to a file
run_test() {
  if [ $# -ne 2 ]; then
    echo "run_test args: pkg_name command" 1>&2
    exit 1
  fi
  pkg_name=$1
  pkg_name=$(echo $pkg_name | tr "/" ".")
  cmd=$2
  if [[ "$SAVELOGS" == "true" ]]; then
    TEST_LOG_PATH="$LOG_DIR/$pkg_name.test.out"
    printf "=== $pkg_name\n"
    printf "  Run $pkg_name\n"
    $cmd > $TEST_LOG_PATH 2>&1
    printf "  Done running $pkg_name\n\tLog path: $TEST_LOG_PATH\n"
    PROC_LOG_PATH="$LOG_DIR/$pkg_name.procs.out"
    printf "  Save $pkg_name proc logs\n"
    ./logs.sh > $PROC_LOG_PATH 2>&1
    printf "  Done saving $pkg_name proc logs\n\tLog path: $PROC_LOG_PATH\n"
  else
    $cmd
  fi
  cleanup
}

check_test_logs() {
  if [ $# -ne 0 ]; then
    echo "check_test_logs expects no args" 1>&2
    exit 1
  fi 
  if [[ "$SAVELOGS" != "true" ]]; then
    return
  fi
  grep -rE "panic|FATAL|FAIL" $LOG_DIR/*.test.out > /dev/null
  if [ $(grep -rwE "panic|FATAL|FAIL" $LOG_DIR/*.test.out > /dev/null; echo $?) -eq 0 ]; then
    echo "!!!!!!!!!! Some tests failed !!!!!!!!!!" | tee $LOG_DIR/summary.out
    grep -rwlE "panic|FATAL|FAIL" $LOG_DIR/*.test.out 2>&1 | tee -a $LOG_DIR/summary.out
  else
    echo "++++++++++ All tests passed ++++++++++" | tee $LOG_DIR/summary.out
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

    for T in path serr linuxsched util/perf sigmap dialproxy session/clnt proxy/ninep sigmaclnt/fslib/reader sigmaclnt/fslib/writer sigmasrv/stats sigmaclnt/fslib util/coordination/semclnt chunk/srv ft/leaderclnt/electclnt dircache sigmasrv/memfssrv/memfs namesrv namesrv/fsetcd sigmaclnt/procclnt proxy/ux proxy/s3 boot/clnt ft/leaderclnt leadertest apps/kv/kvgrp apps/cache/cachegrp/clnt apps/www sigmasrv/memfssrv/sigmapsrv realm/clnt apps/mr apps/imgresize apps/kv apps/hotel apps/socialnetwork benchmarks benchmarks/remote example example_echo_server benchmarks/netperf; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        run_test $T "go test $VERB sigmaos/$T --run TestCompile"
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
        run_test $T "go test $VERB sigmaos/$T"
    done

    #
    # test sessions
    #
    
    for T in session/clnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        run_test $T "go test $VERB sigmaos/$T"
    done

    #
    # test with a kernel with just named
    #

    for T in sigmaclnt/fslib/reader sigmaclnt/fslib/writer sigmasrv/stats dialproxy sigmaclnt/fslib ft/leaderclnt/electclnt dircache; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        run_test $T "go test $VERB -timeout 20m sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL"
    done

    # run_test $sigmapsrv "go test $VERB sigmaos/sigmapsrv -start"  # no perf

    # test memfs
    run_test "memfs/local" "go test $VERB sigmaos/sigmaclnt/fslib -start -path "name/memfs/~local/"  $SPPROXYD $DIALPROXY $REUSEKERNEL"
    run_test "memfs/start" "go test $VERB sigmaos/memfs -start $SPPROXYD $DIALPROXY $REUSEKERNEL"

    #
    # tests a full kernel using root realm
    #

    for T in namesrv util/coordination/semclnt chunk/srv sigmaclnt/procclnt proxy/ux boot/clnt proxy/s3 ft/leaderclnt leadertest apps/kv/kvgrp apps/cache/cachegrp/clnt; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        run_test $T "go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL"
    done

    #
    # test ninep proxy with just named and full kernel
    #

    for T in proxy/ninep; do
        if ! [ -z "$SKIPTO" ]; then
          if [[ "$SKIPTO" == "$T" ]]; then
            # Stop skipping
            SKIPTO=""
          else
            # Skip
            continue
          fi
        fi
        run_test $T "go test $VERB sigmaos/$T -start"
    done


    run_test "sigmapsrv/ux" "go test $VERB sigmaos/sigmasrv/memfssrv/sigmapsrv -start -path "name/ux/~local/" -run ReadPerf"
    run_test "sigmapsrv/s3" "go test $VERB sigmaos/sigmasrv/memfssrvsigmapsrv -start -path "name/s3/~local/9ps3/" -run ReadPerf"
    run_test "sigmapsrv/s3pathclnt" "go test $VERB sigmaos/sigmasrv/memfssrvsigmapsrv --withs3pathclnt -start -path "name/s3/~local/9ps3/" -run ReadFilePerfSingle"
    

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
      run_test $T "go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL"
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
          run_test $T "go test $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY -run '${TNAMES[$i]}'"
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
            run_test $T "go test -timeout 20m $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL"
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
            run_test $T "go test -timeout 50m $VERB sigmaos/$T -start $SPPROXYD $DIALPROXY $REUSEKERNEL"
        done
    fi
fi

#
# Container tests (will OOM your machine if you don't have 1:1 memory:swap ratio)
#

if [[ $CONTAINER == "--container" ]] ; then
    run_test $T "go test $VERB sigmaos/scontainer -start"
fi

cleanup

check_test_logs
