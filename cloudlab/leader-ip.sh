#!/bin/bash

usage() {
  echo "Usage: $0" 1>&2
}

while [[ $# -gt 1 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    ;;
  --vpc)
    shift
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"

MAIN_PRIVADDR=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN hostname -I | cut -d " " -f1)

echo $MAIN_PRIVADDR
