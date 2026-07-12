#!/bin/bash

usage() {
  echo "Usage: $0 [--perfdir PERFDIR] [--parallel] [--logs]" 1>&2
}

PARALLEL=""
LOGS_ONLY=""
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
  --logs)
    shift
    LOGS_ONLY="true"
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
vm_ids=`cat servers.txt | cut -d " " -f1` 
vma=($vms)
vm_id_a=($vm_ids)
MAIN="${vma[0]}"

LOG_DIR=/tmp/sigmaos-node-logs
if [ -z "$LOGS_ONLY" ]; then
  mkdir -p $PERF_DIR
fi
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
  vm_hostname="${vm_id_a[$idx]}"
  cmd1="ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm \"/bin/bash -c '~/sigmaos/logs.sh'\" > $LOG_DIR/$vm_hostname-$vm.out 2>&1"
  # zip up performance files.
  cmd2="ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm \"/bin/bash -c 'rm -f /tmp/perf.tar.gz; cd /tmp/sigmaos-perf; tar -czf ../perf.tar.gz .'\""
  # scp performance files.
  cmd3="scp -C -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm:/tmp/perf.tar.gz $PERF_DIR/$vm_hostname-perf.tar.gz"
  # unzip performance files.
  cmd4="tar -xzf $PERF_DIR/$vm_hostname-perf.tar.gz -C $PERF_DIR; rm $PERF_DIR/$vm_hostname-perf.tar.gz"
  # scp the bench.out file.
  cmd5="scp -C -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm:/tmp/bench.out $PERF_DIR/bench.out.$idx"
  idx=$((idx+1)) 
  if [ -z "$PARALLEL" ]; then
    eval "$cmd1"
    if [ -z "$LOGS_ONLY" ]; then
      eval "$cmd2"
      eval "$cmd3"
      eval "$cmd4"
      eval "$cmd5"
    fi
  else
    (
      eval "$cmd1"
      if [ -z "$LOGS_ONLY" ]; then
        eval "$cmd2"
        eval "$cmd3"
        eval "$cmd4"
        eval "$cmd5"
      fi
    ) &
  fi
done
wait

echo -e "\n\n===================="
if [ -z "$LOGS_ONLY" ]; then
  cp -r $LOG_DIR $PERF_DIR/
  echo "Perf results are in $PERF_DIR"
fi
echo "VM logs are in $LOG_DIR"
