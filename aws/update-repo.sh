#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--parallel] [--n N] [--branch BRANCH]" 1>&2
}

VPC=""
REALM=""
N_VM=""
BRANCH="master"
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
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

if [ -z "$VPC" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms; do
  echo "UPDATE: $vm"
  install="
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
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
