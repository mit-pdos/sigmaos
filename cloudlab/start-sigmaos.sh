#!/bin/bash

usage() {
  echo "Usage: $0 [--branch BRANCH] [--reserveMcpu rmcpu] [--pull TAG] [--n N_VM] [--ncores NCORES] [--overlays] [--nonetproxy] [--turbo] [--numfullnode N]" 1>&2
}

VPC=""
N_VM=""
NCORES=4
UPDATE=""
TAG=""
OVERLAYS=""
NUM_FULL_NODE="0"
NETPROXY="--usenetproxy"
TOKEN=""
TURBO=""
RMCPU="0"
BRANCH="master"
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --parallel)
    shift
    ;;
  --vpc)
    shift
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
  --ncores)
    shift
    NCORES=$1
    shift
    ;;
  --turbo)
    shift
    TURBO="--turbo"
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
  --nonetproxy)
    shift
    NETPROXY=""
    ;;
  --numfullnode)
    shift
    NUM_FULL_NODE=$1
    shift
    ;;
  --reserveMcpu)
    shift
    RMCPU="$1"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [ $NCORES -ne 2 ] && [ $NCORES -ne 4 ] && [ $NCORES -ne 8 ] && [ $NCORES -ne 16 ] && [ $NCORES -ne 20 ] && [ $NCORES -ne 32 ] && [ $NCORES -ne 40 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

vms=`cat servers.txt | cut -d " " -f2`
all_vms="$vms"
vma=($vms)
MAIN="${vma[0]}"
MAIN_PRIVADDR=$(./leader-ip.sh)
SIGMASTART=$MAIN
SIGMASTART_PRIVADDR=$MAIN_PRIVADDR
IMGS="arielszekely/sigmauser arielszekely/sigmaos arielszekely/sigmaosbase"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if ! [ -z "$TAG" ]; then
  ./update-repo.sh --parallel --branch $BRANCH
fi

vm_ncores=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN nproc --all)
first_core_off=$NCORES
last_core=$(($vm_ncores - 1))

i=0
for vm in $vms; do
  i=$(($i+1))
  if [ $NUM_FULL_NODE -gt 0 ] && [ $i -gt $NUM_FULL_NODE ]; then
    NODETYPE="minnode"
  else
    NODETYPE="node"
  fi
  echo "starting SigmaOS on $vm nodetype: $NODETYPE!"
  $DIR/setup-for-benchmarking.sh $vm $TURBO
  # Get hostname.
  VM_NAME=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm hostname -s)
  KERNELID="sigma-$VM_NAME-$(echo $RANDOM | md5sum | head -c 3)"
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
  mkdir -p /tmp/sigmaos
  export SIGMADEBUG="$SIGMADEBUG"
  # Turn on all cores
  ./sigmaos/set-cores.sh --set 1 --start 1 --end $last_core > /dev/null
  # If not using all cores, switch some of them off
  if [ $NCORES -ne $vm_ncores ]; then
    ./sigmaos/set-cores.sh --set 0 --start $first_core_off --end $last_core > /dev/null
  fi
  
#  aws s3 --profile sigmaos cp s3://9ps3/img-save/1.jpg ~/
#  aws s3 --profile sigmaos cp s3://9ps3/img-save/6.jpg ~/
#  aws s3 --profile sigmaos cp s3://9ps3/img-save/7.jpg ~/
  if ! [ -f ~/8.jpg ]; then
    aws s3 --profile sigmaos cp s3://9ps3/img-save/8.jpg ~/
  fi

  cd sigmaos
  sudo ./load-apparmor.sh

  echo "$PWD $SIGMADEBUG"
  if [ "${vm}" = "${MAIN}" ]; then
    echo "START DB: ${MAIN_PRIVADDR}"
    ./start-db.sh
    ./make.sh --norace linux
    echo "START NETWORK $MAIN_PRIVADDR"
    ./start-network.sh --addr $MAIN_PRIVADDR
    echo "START ${SIGMASTART} ${SIGMASTART_PRIVADDR} ${KERNELID}"
    if ! docker ps | grep -q etcd ; then
      echo "START etcd"
      ./start-etcd.sh
    fi
    ./start-kernel.sh --boot realm --named ${SIGMASTART_PRIVADDR} --pull ${TAG} --reserveMcpu ${RMCPU} --dbip ${MAIN_PRIVADDR}:4406 --mongoip ${MAIN_PRIVADDR}:4407 ${OVERLAYS} ${NETPROXY} ${KERNELID} 2>&1 | tee /tmp/start.out
#    docker cp ~/1.jpg ${KERNELID}:/home/sigmaos/1.jpg
#    docker cp ~/6.jpg ${KERNELID}:/home/sigmaos/6.jpg
#    docker cp ~/7.jpg ${KERNELID}:/home/sigmaos/7.jpg
    docker cp ~/8.jpg ${KERNELID}:/home/sigmaos/8.jpg
  else
    echo "JOIN ${SIGMASTART} ${KERNELID}"
    ${TOKEN} 2>&1 > /dev/null
    ./start-kernel.sh --boot $NODETYPE --named ${SIGMASTART_PRIVADDR} --pull ${TAG} --dbip ${MAIN_PRIVADDR}:4406 --mongoip ${MAIN_PRIVADDR}:4407 ${OVERLAYS} ${NETPROXY} ${KERNELID} 2>&1 | tee /tmp/join.out
#    docker cp ~/1.jpg ${KERNELID}:/home/sigmaos/1.jpg
#    docker cp ~/6.jpg ${KERNELID}:/home/sigmaos/6.jpg
#    docker cp ~/7.jpg ${KERNELID}:/home/sigmaos/7.jpg
    docker cp ~/8.jpg ${KERNELID}:/home/sigmaos/8.jpg
  fi
ENDSSH
  if [ "${vm}" = "${MAIN}" ]; then
    TOKEN=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm docker swarm join-token worker | grep docker)
  fi   
done
