#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --realm REALM [--n N] " 1>&2
}

N_VM=""
VPC=""
REALM=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
    VPC=$1
    shift
    ;;
  --realm)
    shift
    REALM=$1
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

if [ -z "$VPC" ] || [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/../.env

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
NAMED="${vma[0]}:1111"
export NAMED="${NAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms; do
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  export NAMED="${NAMED}"
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START ${NAMED}"
    (cd ulambda; nohup ./start.sh --realm $REALM > /tmp/start.out 2>&1 < /dev/null &)
  else
    echo "JOIN ${NAMED}"
    (cd ulambda; SIGMAPID=machined-$vm nohup $PRIVILEGED_BIN/realm/machined > /tmp/machined.out 2>&1 < /dev/null &)
  fi
ENDSSH
done
