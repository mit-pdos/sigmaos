#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--pull TAG] [--n N_VM] [--ncores NCORES] [--overlays]" 1>&2
}

VPC=""
N_VM=""
NCORES=4
UPDATE=""
TAG=""
OVERLAYS=""
TOKEN=""
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

if [ -z "$VPC" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [ $NCORES -ne 4 ] && [ $NCORES -ne 2 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

vms_full=$(./lsvpc.py  --privaddr $VPC)
vms=`echo "$vms_full" | grep -w VMInstance | cut -d " " -f 5`
vms_privaddr=`echo "$vms_full" | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
vma_privaddr=($vms_privaddr)
MAIN="${vma[0]}"
MAIN_PRIVADDR="${vma_privaddr[0]}"
SIGMANAMED="${vma[0]}:1111"
IMGS="arielszekely/sigmauser arielszekely/sigmaos arielszekely/sigmaosbase"
#export SIGMANAMED="${SIGMANAMED}"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if [ ! -z "$TAG" ]; then
  ./update-repo.sh --vpc $VPC --parallel
fi

vm_ncores=$(ssh -i key-$VPC.pem ubuntu@$MAIN nproc)

for vm in $vms; do
  echo "starting SigmaOS on $vm!"
  # No benchmarking setup needed for AWS.
  # Get hostname.
  VM_NAME=$(echo "$vms_full" | grep $vm | cut -d " " -f 2)
  KERNELID="sigma-$VM_NAME-$(echo $RANDOM | md5sum | head -c 3)"
  ssh -i key-$VPC.pem ubuntu@$vm /bin/bash <<ENDSSH
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
    ./start-kernel.sh --boot node --named ${SIGMANAMED} --pull ${TAG} --dbip ${MAIN_PRIVADDR}:4406 --mongoip ${MAIN_PRIVADDR}:4407 --jaeger ${MAIN_PRIVADDR} ${OVERLAYS} ${KERNELID} 2>&1 | tee /tmp/join.out
  fi
ENDSSH
 if [ "${vm}" = "${MAIN}" ]; then
     TOKEN=$(ssh -i key-$VPC.pem ubuntu@$vm docker swarm join-token worker | grep docker)
 fi   
done
