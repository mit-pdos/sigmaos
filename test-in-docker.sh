#!/bin/bash

usage() {
  echo "Usage: $0 [--rebuildtester]" 1>&2
}

REBUILD_TESTER="false"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

testercid=$(docker ps -a | grep -w "sig-tester" | cut -d " " -f1)

ETCD_CTR_NAME=etcd-tester
TESTER_NETWORK=sigmanet-testuser
if ! docker ps | grep -q $ETCD_CTR_NAME ; then
  DATA_DIR="etcd-tester-data"
  if ! docker volume ls | grep -q $DATA_DIR; then
      echo "create vol"
      docker volume create --name $DATA_DIR
  fi
  docker run -d \
      --name $ETCD_CTR_NAME \
      --env ALLOW_NONE_AUTHENTICATION=yes \
      --network $TESTER_NETWORK \
      bitnami/etcd:latest
else
  # delete all keys from etcd
  docker exec $ETCD_CTR_NAME etcdctl del --prefix ''
fi

if [[ $REBUILD_TESTER == "true" ]]; then
  if ! [ -z "$testercid" ]; then
    echo "========== Stopping old tester container $testercid =========="
    docker stop $testercid
    # Reset tester container ID
    testercid=""
  fi
fi

ROOT=$(pwd)
BUILD_LOG=/tmp/sigmaos-build
mkdir -p $BUILD_LOG

if [ -z "$testercid" ]; then
  # Build tester
  echo "========== Build tester image =========="
  DOCKER_BUILDKIT=1 docker build --progress=plain -f tester.Dockerfile -t sig-tester . 2>&1 | tee $BUILD_LOG/sig-tester.out
  echo "========== Done building tester =========="
  # Start tester
  echo "========== Starting tester container =========="
  mkdir -p /tmp/sigmaos-bin
  docker run --rm -d -it \
    --name sig-tester \
    --network $TESTER_NETWORK \
    --mount type=bind,src=$ROOT,dst=/home/sigmaos/ \
    --mount type=bind,src=$HOME/.aws,dst=/home/sigmaos/.aws \
    --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
    --mount type=bind,src=/sys/fs/cgroup,dst=/sys/fs/cgroup \
    --mount type=bind,src=/tmp/sigmaos,dst=/tmp/sigmaos \
    --mount type=bind,src=/tmp/sigmaos-bin/,dst=/tmp/sigmaos-bin \
    --mount type=bind,src=/tmp/spproxyd,dst=/tmp/spproxyd \
    --mount type=bind,src=/tmp/sigmaos-data,dst=/tmp/sigmaos-data \
    --mount type=bind,src=/tmp/sigmaos-perf,dst=/tmp/sigmaos-perf \
    sig-tester
  testercid=$(docker ps -a | grep -w "sig-tester" | cut -d " " -f1)
  until [ "`docker inspect -f {{.State.Running}} $testercid`"=="true" ]; do
      echo -n "." 1>&2
      sleep 0.1;
  done
  echo "========== Done starting tester ========== "
fi

# Clean the test cache
docker exec \
  -it $(docker ps -a | grep sig-tester | cut -d " " -f1) \
  go clean -testcache

SPKG=sigmaclnt/procclnt
TNAME=WaitExitSimpleSingleBE

ETCD_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $ETCD_CTR_NAME)

# Run the test
docker exec \
  --env SIGMADEBUG="$SIGMADEBUG" \
  -it $(docker ps -a | grep sig-tester | cut -d " " -f1) \
  go test -v sigmaos/$SPKG --run $TNAME \
  --start \
  --homedir $HOME \
  --projectroot /home/arielck/sigmaos \
  --etcdIP $ETCD_IP \
  --netname $TESTER_NETWORK
