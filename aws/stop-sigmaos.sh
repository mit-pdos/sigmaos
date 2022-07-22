#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--n N]" 1>&2
}

VPC=""
N_VM=""
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

DIR=$(dirname $0)
. $DIR/../.env

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms
do
    echo "stop: $vm"
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
    (cd ulambda; ./stop.sh)
    rm -rf $UXROOT
ENDSSH
done
