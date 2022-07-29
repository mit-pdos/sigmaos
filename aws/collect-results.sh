#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--parallel]" 1>&2
}

VPC=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
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
MAIN="${vma[0]}"

RUN_NUM=$(date +%s)

PERF_DIR=../benchmarks/results/$RUN_NUM
LOG_DIR=/tmp
mkdir $PERF_DIR

for vm in $vms; do
  echo "scp: $vm"
  if [ $vm == $MAIN ]; then
    outfile="/tmp/start.out"
  else
    outfile="/tmp/machined.out"
  fi
  # scp machined.out files.
  cmd1="scp -i key-$VPC.pem ubuntu@$vm:$outfile $LOG_DIR/$vm.out"
  # scp performance files.
  cmd2="scp -i key-$VPC.pem ubuntu@$vm:/tmp/sigmaos/perf-output/* $PERF_DIR"
  if [ -z "$PARALLEL" ]; then
    eval "$cmd1"
    eval "$cmd2"
  else
    (
      eval "$cmd1"
      eval "$cmd2"
    ) &
  fi
done
wait

echo -e "\n\n===================="
echo "Perf results are in $PERF_DIR"
echo "VM logs are in $LOG_DIR"
