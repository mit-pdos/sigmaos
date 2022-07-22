#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC" 1>&2
}

VPC=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
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

RUN_NUM=$(date +%s)

mkdir ../benchmarks/results/$RUN_NUM

for vm in $vms; do
  echo "scp: $vm"
  scp -i key-$VPC.pem ubuntu@$vm:/tmp/sigmaos/perf-output/* ../benchmarks/results/$RUN_NUM
  scp -i key-$VPC.pem ubuntu@$vm:/tmp/machined.out /tmp/$vm.out
done
