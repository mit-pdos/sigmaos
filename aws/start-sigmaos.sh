#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--update] [--ncores NCORES]" 1>&2
}

VPC=""
N_VM=""
NCORES=4
UPDATE=""
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
  --ncores)
    shift
    NCORES=$1
    shift
    ;;
  --update)
    UPDATE="yes"
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

if [ $NCORES -ne 4 ] && [ $NCORES -ne 2 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

vms=`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`

vma=($vms)
MAIN="${vma[0]}"
SIGMANAMED="${vma[0]}:1111"
IMGS="sigmauser sigmaos sigmaosbase"
#export SIGMANAMED="${SIGMANAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if [ ! -z "$UPDATE" ]; then
    # XXX build on one and then pull container?
    ./update-repo.sh --vpc $VPC --parallel
    for vm in $vms; do
        echo $vm
        ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  cd ulambda
  ./build.sh --target aws
ENDSSH
    done

fi

for vm in $vms; do
    echo $vm
    ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
  mkdir -p /tmp/sigmaos
  export SIGMADBADDR="10.0.102.10:3306"
  # export SIGMANAMED="${SIGMANAMED}"
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

  cd ulambda
  echo $PWD

  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START ${SIGMANAMED}"
    ./start-db.sh
    ./start.sh --boot all --host
  else
    echo "JOIN ${SIGMANAMED}"
    ./start.sh --boot node --named ${SIGMANAMED} --host
  fi
ENDSSH
done
