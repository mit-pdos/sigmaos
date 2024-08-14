#!/bin/bash

usage() {
  echo "Usage: $0 [--perfdir PERFDIR] [--parallel]" 1>&2
}

PARALLEL=""
PERF_DIR=../benchmarks/results/$(date +%s)
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    shift
    ;;
  --perfdir)
    shift
    PERF_DIR=$1
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
MAIN="${vma[0]}"

LOG_DIR=/tmp/sigmaos-node-logs
mkdir -p $PERF_DIR
# Remove old logs
rm $LOG_DIR/*.out
mkdir -p $LOG_DIR

idx=0
for vm in $vms; do
  echo "scp: $vm"
  if [ $vm == $MAIN ]; then
    outfile="/tmp/start.out"
  else
    outfile="/tmp/join.out"
  fi
  # read log files.
  cmd1="ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm \"/bin/bash -c '~/sigmaos/logs.sh --merge'\" > $LOG_DIR/$vm.out 2>&1"
  # scp performance files.
  cmd2="scp -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm:/tmp/sigmaos-perf/* $PERF_DIR"
  # scp the bench.out file.
  cmd3="scp -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm:/tmp/bench.out $PERF_DIR/bench.out.$idx"
  idx=$((idx+1)) 
  if [ -z "$PARALLEL" ]; then
    eval "$cmd1"
    eval "$cmd2"
    eval "$cmd3"
  else
    (
      eval "$cmd1"
      eval "$cmd2"
      eval "$cmd3"
    ) &
  fi
done
wait

cp -r $LOG_DIR $PERF_DIR/

echo -e "\n\n===================="
echo "Perf results are in $PERF_DIR"
echo "VM logs are in $LOG_DIR"
