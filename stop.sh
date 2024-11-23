#!/bin/bash

usage() {
  echo "Usage: $0 [--parallel] [--nopurge] [--skipdb]" 1>&2
}

PARALLEL=""
PURGE="true"
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

if mount | grep -q 9p; then
    echo "umount /mnt/9p"
    ./umount.sh
fi

pgrep -x npproxyd > /dev/null && killall -9 npproxyd
pgrep -x spproxyd > /dev/null && killall -9 spproxyd

sudo rm -f /tmp/spproxyd/spproxyd.sock
sudo rm -f /tmp/spproxyd/spproxyd-dialproxy.sock
sudo rm -f /tmp/spproxyd/spproxyd-pyproxy.sock

if docker ps -a | grep -qE 'sigma|uprocd|bootkerne|kernel-'; then
  for container in $(docker ps -a | grep -E 'sigma|uprocd|bootkerne|kernel-' | cut -d ' ' -f1) ; do
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

sudo rm -rf /tmp/sigmaos-bin
sudo rm -rf /tmp/sigmaos-kernel-start-logs

# delete all keys from etcd
docker exec etcd-server etcdctl del --prefix ''
