#!/bin/bash

usage() {
  echo "Usage: $0 --svc SVC_NAME" 1>&2
}

SVC_NAME=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    ;;
  --vpc)
    shift
    shift
    ;;
  --svc)
    shift
    SVC_NAME=$1
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

if [ -z "$SVC_NAME" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh
vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"

ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN <<ENDSSH
  kubectl get svc | grep -wE "^$SVC_NAME" | tr -s " " | cut -d " " -f 3
ENDSSH
