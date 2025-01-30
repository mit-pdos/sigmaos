#!/bin/bash

usage() {
  echo "Usage: $0 [--parallel] [--nopurge] [--skipdb] [--all]" 1>&2
}

PARALLEL=""
PURGE="true"
ALL="false"
SKIPDB="false"
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    PARALLEL="--parallel"
    ;;
  --nopurge)
    shift
    PURGE=""
    ;;
  --all)
    shift
    ALL="true"
    ;;
  --skipdb)
    shift
    SKIPDB="true"
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done

ROOT=$(dirname $(realpath $0))
source $ROOT/env/env.sh

TMP_BASE="/tmp"
ETCD_CTR_NAME="etcd-server"
USER_IMAGE_NAME="sigmauser"
KERNEL_IMAGE_NAME="sigmaos"
DB_IMAGE_NAME="sigmadb"
MONGO_IMAGE_NAME="sigmamongo"
if ! [ -z "$SIGMAUSER" ]; then
  TMP_BASE="${TMP_BASE}/$SIGMAUSER"
  ETCD_CTR_NAME="etcd-tester-${SIGMAUSER}"
  if [[ "$ALL" == "false" ]]; then
    USER_IMAGE_NAME="$USER_IMAGE_NAME-$SIGMAUSER"
    KERNEL_IMAGE_NAME="$KERNEL_IMAGE_NAME-$SIGMAUSER"
    DB_IMAGE_NAME=$DB_IMAGE_NAME-$SIGMAUSER
    MONGO_IMAGE_NAME=$MONGO_IMAGE_NAME-$SIGMAUSER
  fi
fi

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    ./umount.sh
fi

pgrep -x npproxyd > /dev/null && killall -9 npproxyd
pgrep -x spproxyd > /dev/null && killall -9 spproxyd
pgrep -x start-kernel.sh > /dev/null && killall -9 start-kernel.sh

sudo rm -f $TMP_BASE/spproxyd/spproxyd.sock
sudo rm -f $TMP_BASE/spproxyd/spproxyd-dialproxy.sock
if [[ "$ALL" == "true" ]]; then
  sudo rm -f /tmp/spproxyd/spproxyd.sock
  sudo rm -f /tmp/spproxyd/spproxyd-dialproxy.sock
fi

if docker ps -a | grep -qE "$USER_IMAGE_NAME|$KERNEL_IMAGE_NAME|$DB_IMAGE_NAME|$MONGO_IMAGE_NAME"; then
  for container in $(docker ps -a | grep -E "$USER_IMAGE_NAME|$KERNEL_IMAGE_NAME|$DB_IMAGE_NAME|$MONGO_IMAGE_NAME" | cut -d ' ' -f1) ; do
    # Optionally skip DB shutdown
    if [ "$SKIPDB" == "true" ]; then
      cname=$(docker ps -a | grep $container | cut -d ' ' -f4)
      if [ "$cname" == "mariadb" ] || [ "$cname" == "mongo:4.4.6" ]; then
        echo "skipping stop db $cname"
        continue
      fi
    fi
    stop="
      docker stop $container 
      docker rm $container
    "
    if [ -z "$PARALLEL" ]; then
      eval "$stop"
    else
      (
        eval "$stop"
      ) &
    fi
  done
fi

wait

if ! [ -z $PURGE ]; then
  yes | docker system prune
  yes | docker volume prune
fi

sudo rm -rf $TMP_BASE/sigmaos-bin/*
sudo rm -rf $TMP_BASE/sigmaos-kernel-start-logs

# delete all keys from etcd
if docker ps | grep -q $ETCD_CTR_NAME ; then
  docker exec $ETCD_CTR_NAME etcdctl del --prefix ''
fi

if [[ "$ALL" == "true" ]]; then
  if docker ps | grep -q etcd-server ; then
    docker exec etcd-server etcdctl del --prefix ''
  fi
fi
