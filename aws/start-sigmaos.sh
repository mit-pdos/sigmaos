#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC [--branch BRANCH] [--reserveMcpu rmcpu] [--pull TAG] [--n N_VM] [--ncores NCORES] [--overlays] [--nonetproxy] [--turbo] [--nodetype node|minnode]" 1>&2
}

VPC=""
N_VM=""
NCORES=4
UPDATE=""
TAG=""
OVERLAYS=""
NODETYPE="node"
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
  --reserveMcpu)
    shift
    RMCPU="$1"
  	shift
    ;;
  --nodetype)
    shift
    NODETYPE=$1
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

if [ $NCORES -ne 16 ] && [ $NCORES -ne 4 ] && [ $NCORES -ne 2 ]; then
  echo "Bad ncores $NCORES"
  exit 1
fi

if [ $NODETYPE != "node" ] && [ $NODETYPE != "minnode" ]; then
  echo "Bad node type\"$NODETYPE\""
  exit 1
fi

vms_full=$(./lsvpc.py --privaddr $VPC | grep '.amazonaws.com')
vms=`echo "$vms_full" | grep -w VMInstance | cut -d " " -f 5`
all_vms="$vms"
vms_privaddr=`echo "$vms_full" | grep -w VMInstance | cut -d " " -f 6`

vma=($vms)
vma_privaddr=($vms_privaddr)
MAIN="${vma[0]}"
MAIN_PRIVADDR="${vma_privaddr[0]}"
SIGMASTART=$MAIN
SIGMASTART_PRIVADDR=$MAIN_PRIVADDR
IMGS="arielszekely/sigmauser arielszekely/sigmaos arielszekely/sigmaosbase"

if ! [ -z "$N_VM" ]; then
  vms=${vma[@]:0:$N_VM}
fi

if ! [ -z "$TAG" ]; then
  ./update-repo.sh --vpc $VPC --parallel --branch $BRANCH
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
  export SIGMAPERF="$SIGMAPERF"
  export SIGMADEBUG="$SIGMADEBUG"
  if [ $NCORES -eq 2 ]; then
    ./sigmaos/set-cores.sh --set 0 --start 2 --end $vm_ncores > /dev/null
    echo "ncores:"
    nproc
  else
    ./sigmaos/set-cores.sh --set 1 --start 2 --end 3 > /dev/null
    echo "ncores:"
    nproc
  fi
  
#  if ! [ -f ~/1.jpg ]; then
#    aws s3 --profile sigmaos cp s3://9ps3/img-save/1.jpg ~/
#  fi
#  if ! [ -f ~/6.jpg ]; then
#    aws s3 --profile sigmaos cp s3://9ps3/img-save/6.jpg ~/
#  fi
#  if ! [ -f ~/7.jpg ]; then
#    aws s3 --profile sigmaos cp s3://9ps3/img-save/7.jpg ~/
#  fi
  if ! [ -f ~/8.jpg ]; then
    aws s3 --profile sigmaos cp s3://9ps3/img-save/8.jpg ~/
  fi
#
#  # Download wiki dataset
#  mkdir -p /tmp/sigmaos-data
#  if ! [ -d /tmp/sigmaos-data/wiki-20G ]; then 
#    mkdir /tmp/sigmaos-data/wiki-20G
#    aws s3 --profile sigmaos cp s3://9ps3/wiki-20G/enwiki-latest-pages-articles-multistream-augmented.xml /tmp/sigmaos-data/wiki-20G/enwiki-latest-pages-articles-multistream-augmented.xml
#  fi

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
#    docker cp ~/8.jpg ${KERNELID}:/home/sigmaos/8.jpg
  fi
ENDSSH
 if [ "${vm}" = "${MAIN}" ]; then
     TOKEN=$(ssh -i key-$VPC.pem ubuntu@$vm docker swarm join-token worker | grep docker)
 fi   
done
