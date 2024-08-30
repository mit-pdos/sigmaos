#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --path APP_PATH" 1>&2
}

APP_PATH=""
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
  --path)
    shift
    APP_PATH=$1
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

if [ -z "$APP_PATH" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"

ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN <<ENDSSH
  kubectl delete -Rf $APP_PATH > /dev/null 2>&1
ENDSSH
