#!/bin/bash

usage() {
  echo "Usage: $0 [--parallel] [--n N] [--branch BRANCH]" 1>&2
}

REALM=""
N_VM=""
BRANCH="master"
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --n)
    shift
    N_VM=$1
    shift
    ;;
  --branch)
    shift
    BRANCH=$1
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

LOGIN="arielck"
DIR=$(dirname $0)

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms; do
  echo "UPDATE: $vm"
  install="
    ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
      ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull > /tmp/git.out 2>&1 ; git checkout $BRANCH; git pull >> /tmp/git.out 2>&1 )'
ENDSSH"
  if [ -z "$PARALLEL" ]; then
    eval "$install"
  else
  (
    eval "$install"
  ) &
  fi
done
wait
