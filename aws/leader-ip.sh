#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC" 1>&2
}

VPC=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    ;;
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

vms=`./lsvpc.py $VPC --privaddr | grep '.amazonaws.com' | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
MAIN_PRIVADDR="${vma[0]}"

echo $MAIN_PRIVADDR
