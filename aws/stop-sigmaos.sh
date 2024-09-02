#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--n N] [--parallel]" 1>&2
}

VPC=""
N_VM=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
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

if [ -z "$VPC" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms
do
  echo "stop: $vm"
  stop="
      ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
        (cd sigmaos; ./stop-benchmarks.sh; ./stop.sh; ./stop-etcd.sh)
        sudo killall logs.sh > /dev/null 2>&1
        rm -rf /tmp/sigmaos-perf > /dev/null 2>&1
        rm /tmp/bench.out > /dev/null 2>&1
        rm /tmp/start.out > /dev/null 2>&1
        rm /tmp/make.out > /dev/null 2>&1
        rm /tmp/machine.out > /dev/null 2>&1
        yes | docker system prune
        yes | docker volume prune
        docker swarm leave --force
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
