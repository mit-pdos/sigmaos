#!/bin/bash

usage() {
  echo "Usage: $0 username [--pull TAG] [--n N_VM] [--ncores NCORES] [--overlays]" 1>&2
}

N_VM=""
NCORES=4
UPDATE=""
TAG=""
OVERLAYS=""
TOKEN=""
LOGIN=$1
shift
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --vpc)
    shift
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
  --pull)
    shift
    TAG=$1
    shift
    ;;
  --overlays)
    shift
    OVERLAYS="--overlays"
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

if [ $NCORES -ne 4 ] && [ $NCORES -ne 2 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

DIR=$(dirname $0)

vms=`cat servers.txt | cut -d " " -f2` 

vma=($vms)
MAIN="${vma[0]}"
MAIN_PRIVADDR=$(./leader-ip.sh $LOGIN)
SIGMANAMED="${vma[0]}:1111"
IMGS="arielszekely/sigmauser arielszekely/sigmaos arielszekely/sigmaosbase"
#export SIGMANAMED="${SIGMANAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if [ ! -z "$TAG" ]; then
  ./update-repo.sh $LOGIN --parallel
fi

vm_ncores=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN nproc)

for vm in $vms; do
  echo "starting SigmaOS on $vm!"
  $DIR/setup-for-benchmarking.sh $LOGIN@$vm
  # Get hostname.
  VM_NAME=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm hostname -s)
  KERNELID="sigma-$VM_NAME-$(echo $RANDOM | md5sum | head -c 3)"
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
  mkdir -p /tmp/sigmaos
  export SIGMADEBUG="$SIGMADEBUG"
  if [ $NCORES -eq 2 ]; then
    ./ulambda/set-cores.sh --set 0 --start 2 --end $vm_ncores > /dev/null
    echo "ncores:"
    nproc
  else
    ./ulambda/set-cores.sh --set 1 --start 2 --end 3 > /dev/null
    echo "ncores:"
    nproc
  fi

  cd ulambda

  echo "$PWD $SIGMADEBUG"
  if [ "${vm}" = "${MAIN}" ]; then 
    echo "START ${SIGMANAMED} ${KERNELID}"
    ./make.sh --norace linux
    ./start-network.sh --addr $MAIN_PRIVADDR
    ./start-db.sh
    ./start-jaeger.sh
    ./start-kernel.sh --boot realm --pull ${TAG} --jaeger ${MAIN_PRIVADDR} ${OVERLAYS} ${KERNELID} 2>&1 | tee /tmp/start.out
  else
    echo "JOIN ${SIGMANAMED} ${KERNELID}"
     ${TOKEN} 2>&1 > /dev/null
    ./start-kernel.sh --boot node --named ${SIGMANAMED} --pull ${TAG} --jaeger ${MAIN_PRIVADDR} ${OVERLAYS} ${KERNELID} 2>&1 | tee /tmp/join.out
  fi
ENDSSH
 if [ "${vm}" = "${MAIN}" ]; then
     TOKEN=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm docker swarm join-token worker | grep docker)
 fi   
done
