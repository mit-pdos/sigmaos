#!/bin/bash

FIRST=""
VPC=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  -n)
    shift
    FIRST=$1
    shift
    ;;
  *)
    VPC=$1
    shift
    break
    ;;
  esac
done

if [ -z "$VPC" ]; then
    echo "Usage:[-n <n>] vpc-id"
    exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
NAMED="${vma[0]}:1111"
export NAMED="${NAMED}"

if ! [ -z "$FIRST" ]; then
  vms=${vma[@]:0:$FIRST}
fi

for vm in $vms; do
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  mkdir -p /tmp/ulambda/
  export NAMED="${NAMED}"
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START ${NAMED}"
    (cd ulambda; nohup ./start.sh > /tmp/start.out 2>&1 < /dev/null &)
  else
    echo "JOIN ${NAMED}"
    (cd ulambda;  nohup bin/realm/noded . $vm > /tmp/noded.out 2>&1 < /dev/null &)
  fi
ENDSSH
done
