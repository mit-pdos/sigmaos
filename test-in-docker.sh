#!/bin/bash

usage() {
  echo "Usage: $0 --race --pkg PKG [--run TEST] [--args ARGS] [--no-start] [--rebuildtester]" 1>&2
}

TNAME="Test"
SPKG=""
ARGS=""
START="--start"
REBUILD_TESTER="false"
RACE=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --run)
      shift
      TNAME="$1" 
      shift
      ;;
  --no-start)
      shift
      START="" 
      ;;
  --pkg)
      shift
      SPKG="$1" 
      shift
      ;;
  --race)
      shift
      RACE="--race" 
      ;;
  --args)
      shift
      ARGS="$1" 
      shift
      ;;
  --rebuildtester)
    shift
    REBUILD_TESTER="true"
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
   echo "unexpected argument $1"
   usage
   exit 1
  esac
done

if [ -z "$SPKG" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

ROOT=$(dirname $(realpath $0))
source $ROOT/env/env.sh

if [ -z "$SIGMAUSER" ] || [[ "$SIGMAUSER" == "NOT_SET" ]] ; then
  echo "SIGMAUSER must be set in $ROOT/env/env.sh in order to run tests in docker"
  exit 1
fi

TMP_BASE="/tmp"
TESTER_NAME="sig-tester"
TESTER_NETWORK="sigmanet-testuser"
ETCD_CTR_NAME="etcd-tester"
if ! [ -z "$SIGMAUSER" ]; then
  TESTER_NAME=$TESTER_NAME-$SIGMAUSER
  TESTER_NETWORK=$TESTER_NETWORK-$SIGMAUSER
  ETCD_CTR_NAME=$ETCD_CTR_NAME-$SIGMAUSER
  TMP_BASE=$TMP_BASE/$SIGMAUSER
fi
HOST_BIN_CACHE="${TMP_BASE}/sigmaos-bin"
DATA_DIR="${TMP_BASE}/sigmaos-data"
PERF_DIR="${TMP_BASE}/sigmaos-perf"
KERNEL_DIR="${TMP_BASE}/sigmaos"
SPPROXY_DIR="${TMP_BASE}/spproxyd"
BUILD_LOG="${TMP_BASE}/sigmaos-build"

mkdir -p $BUILD_LOG
mkdir -p $DATA_DIR
mkdir -p $PERF_DIR
mkdir -p $KERNEL_DIR
mkdir -p $SPPROXY_DIR

# Create the network for the user
$ROOT/create-net.sh $TESTER_NETWORK

# Start up etcd, if it isn't already running
if ! docker ps -a | grep -qw $ETCD_CTR_NAME ; then
  $ROOT/start-etcd.sh --testindocker
fi

testercid=$(docker ps -a | grep -E " $TESTER_NAME " | cut -d " " -f1)

if [[ $REBUILD_TESTER == "true" ]]; then
  if ! [ -z "$testercid" ]; then
    echo "========== Stopping old tester container $testercid =========="
    docker stop $testercid
    # Reset tester container ID
    testercid=""
  fi
fi

mkdir -p $BUILD_LOG

if [ -z "$testercid" ]; then
  # Build tester
  echo "========== Build tester image =========="
  DOCKER_BUILDKIT=1 docker build --progress=plain -f docker/tester.Dockerfile -t $TESTER_NAME . 2>&1 | tee $BUILD_LOG/sig-tester.out
  echo "========== Done building tester =========="
  # Start tester
  echo "========== Starting tester container =========="
  mkdir -p $HOST_BIN_CACHE
  docker run --rm -d -it \
    --name $TESTER_NAME \
    --network $TESTER_NETWORK \
    --mount type=bind,src=$ROOT,dst=/home/sigmaos/ \
    --mount type=bind,src=$HOME/.aws,dst=/home/sigmaos/.aws \
    --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
    --mount type=bind,src=/sys/fs/cgroup,dst=/sys/fs/cgroup \
    --mount type=bind,src=$HOST_BIN_CACHE,dst=$HOST_BIN_CACHE \
    --mount type=bind,src=$SPPROXY_DIR,dst=/tmp/spproxyd \
    --mount type=bind,src=$KERNEL_DIR,dst=$KERNEL_DIR \
    --mount type=bind,src=$DATA_DIR,dst=$DATA_DIR \
    --mount type=bind,src=$PERF_DIR,dst=$PERF_DIR \
    $TESTER_NAME 
  testercid=$(docker ps -a | grep -E " $TESTER_NAME " | cut -d " " -f1)
  until [ "`docker inspect -f {{.State.Running}} $testercid`"=="true" ]; do
      echo -n "." 1>&2
      sleep 0.1;
  done
  echo "========== Done starting tester ========== "
fi

# Clean the test cache
docker exec \
  -it $testercid \
  go clean -testcache

ETCD_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $ETCD_CTR_NAME)

# Run the test
docker exec \
  --env SIGMADEBUG="$SIGMADEBUG" \
  --env SIGMADEBUGPROCS="$SIGMADEBUGPROCS" \
  -it $testercid \
  go test -v $RACE sigmaos/$SPKG --run $TNAME \
  --user $SIGMAUSER \
  --homedir $HOME \
  --projectroot $ROOT \
  --etcdIP $ETCD_IP \
  --netname $TESTER_NETWORK \
  $START \
  $ARGS
