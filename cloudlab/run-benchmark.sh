#!/bin/bash

usage() {
  echo "Usage: $0 --command COMMAND [--vm VM]" 1>&2
}

VPC=""
VM=0
COMMAND=""
while [[ $# -gt 0 ]]; do
  case $1 in
  --parallel)
    shift
    ;;
  --vpc)
    shift
    shift
    ;;
  --command)
    shift
    COMMAND=$1
    shift
    ;;
  --vm)
    shift
    VM=$1
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


if [ -z "$COMMAND" ] || [ $# -gt 0 ]; then
  usage
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2` 
vma=($vms)
MAIN="${vma[0]}"

SSHVM="${vma[$VM]}"

echo "Run [$SSHVM]: $COMMAND"
ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM <<ENDSSH
  # Make sure swap is off on the benchmark machines.
  sudo swapoff -a
  cd sigmaos
  source ./env/env.sh
  export SIGMAPERF="KVCLERK_TPT;MRMAPPER_TPT;MRREDUCER_TPT;HOTEL_WWW_TPT;TEST_TPT;BENCH_TPT;THUMBNAIL_TPT;"
  $COMMAND
ENDSSH
