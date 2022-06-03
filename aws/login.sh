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

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)

MAIN="${vms[0]}"

echo "LOGIN" ${MAIN}

ssh -i key-$VPC.pem -L 1110:localhost:1110 ubuntu@${MAIN} sleep 999999999 
