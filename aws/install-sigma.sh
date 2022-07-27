#!/bin/bash

usage() {
  echo "Usage: $0 [-n N] --vpc VPC --realm REALM [--parallel]" 1>&2
}

VPC=""
REALM=""
N_VM=""
PARALLEL=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  -n)
    shift
    N_VM=$1
    shift
    ;;
  --realm)
    shift
    REALM=$1
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

if [ -z "$VPC" ] || [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms; do
  echo "INSTALL: $vm"
  install="
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
      ssh-agent bash -c 'ssh-add ~/.ssh/aws-ulambda; (cd ulambda; git pull > /tmp/git.out 2>&1 )'
      (cd ulambda; ./stop.sh; ./install.sh --from s3 --realm $REALM)
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
