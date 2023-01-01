#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --realm REALM [--n N] [--ncores NCORES]" 1>&2
}

VPC=""
REALM=""
N_VM=""
NCORES=4
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
  --ncores)
    shift
    NCORES=$1
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

if [ $NCORES -ne 4 ] && [ $NCORES -ne 2 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

DIR=$(dirname $0)
. $DIR/../.env

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
SIGMANAMED="${vma[0]}:1111"
export SIGMANAMED="${SIGMANAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

for vm in $vms; do
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  export SIGMADBADDR="10.0.102.10:3306"
  export SIGMANAMED="${SIGMANAMED}"
#  export SIGMADEBUG="REALMMGR;SIGMAMGR;REALMMGR_ERR;SIGMAMGR_ERR;NODED;NODED_ERR;MACHINED;MACHINED_ERR;"
  if [ $NCORES -eq 2 ]; then
    ./ulambda/set-cores.sh --set 0 --start 2 --end 3 > /dev/null
    echo "ncores:"
    nproc
  else
    ./ulambda/set-cores.sh --set 1 --start 2 --end 3 > /dev/null
    echo "ncores:"
    nproc
  fi
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START ${SIGMANAMED}"
    (cd ulambda; nohup ./start.sh --realm $REALM > /tmp/start.out 2>&1 < /dev/null &)
  else
    echo "JOIN ${SIGMANAMED}"
    (cd ulambda; SIGMAPID=machined-$vm nohup $PRIVILEGED_BIN/realm/machined > /tmp/machined.out 2>&1 < /dev/null &)
  fi
ENDSSH
done
