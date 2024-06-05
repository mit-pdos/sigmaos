#!/bin/bash

usage() {
  echo "Usage: $0 [--n N] [--parallel]" 1>&2
}

N_VM=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    shift
    ;;
  --n)
    shift
    N_VM=$1
    shift
    ;;
  --parallel)
    shift
    PARALLEL="--parallel"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 
vma=($vms)

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms
do
  echo "stop: $vm"
  stop="
      ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
        (cd sigmaos; ./stop-benchmarks.sh; ./stop.sh; ./stop-etcd.sh)
        rm -rf /tmp/sigmaos-perf > /dev/null 2>&1
        rm /tmp/bench.out > /dev/null 2>&1
        rm /tmp/start.out > /dev/null 2>&1
        rm /tmp/make.out > /dev/null 2>&1
        rm /tmp/machine.out > /dev/null 2>&1
        yes | docker system prune
        yes | docker volume prune
#        docker swarm leave --force
ENDSSH"
  if [ -z "$PARALLEL" ]; then
    eval "$stop"
  else
    (
      eval "$stop"
    ) &
  fi
done
wait
