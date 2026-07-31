#!/bin/bash

usage() {
  echo "Usage: $0 [--branch BRANCH] [--reserveMcpu rmcpu] [--pull TAG] [--n N_VM] [--ncores NCORES] [--nodialproxy] [--usegvisor] [--turbo] [--numfullnode N] [--numbeschednode N] [--reload-gvisor-ctr] [--input-data \"DATASET...\"]" 1>&2
  echo "" 1>&2
  echo "  --input-data  Space-separated S3 prefixes to stage on every node before" 1>&2
  echo "                starting its kernel, so that jobs can read them from their" 1>&2
  echo "                local UX server (see download-input-data.sh)." 1>&2
}

VPC=""
N_VM=""
NCORES=4
UPDATE=""
TAG=""
NUM_FULL_NODE="0"
NUM_BESCHED_NODE="0"
DIALPROXY="--usedialproxy"
GVISOR=""
RELOAD_GVISOR="false"
TOKEN=""
TURBO=""
RMCPU="0"
BRANCH="master"
INPUT_DATA=""
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
  --nodialproxy)
    shift
    DIALPROXY=""
    ;;
  --reload-gvisor-ctr)
    shift
    RELOAD_GVISOR="true"
    ;;
  --usegvisor)
    shift
    GVISOR="--usegvisor"
    ;;
  --numfullnode)
    shift
    NUM_FULL_NODE=$1
    shift
    ;;
  --numbeschednode)
    shift
    NUM_BESCHED_NODE=$1
    shift
    ;;
  --reserveMcpu)
    shift
    RMCPU="$1"
  	shift
    ;;
  --input-data)
    shift
    INPUT_DATA="$1"
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

if [ $(($NUM_FULL_NODE + $NUM_BESCHED_NODE)) -gt $N_VM ]; then
  echo "Error: NUM_FULL_NODE + NUM_BESCHED_NODE > N_VM"
  exit 1
fi

if [ $N_VM == 1 ] && [ $NUM_BESCHED_NODE -gt 0 ]; then
  echo "Error: N_VM == 1 but NUM_BESCHED_NODE > 0"
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

LEADER_NODE="realm"
FULL_NODE="node"
BESCHED_NODE="beschednode"
if [ $NUM_BESCHED_NODE -gt 0 ]; then
  LEADER_NODE="realm_no_besched"
  FULL_NODE="node_no_besched"
fi

vm_ncores=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$MAIN nproc --all)
first_core_off=$NCORES
last_core=$(($vm_ncores - 1))

# Nodes 1..SEQ_UNTIL are started one at a time, in order; the rest start in
# parallel. The leader has to be first: it brings up the db, the swarm overlay
# network, etcd and the named every other node joins, and it is the source of
# the swarm join token the followers use. The besched-only nodes follow it, in
# order. Everything after that only joins what already exists, so there is no
# ordering among them.
SEQ_UNTIL=$(($NUM_BESCHED_NODE + 1))

# Exit status per node, so that a failure doesn't disappear into the interleaved
# output of the parallel starts.
STATUS_DIR=$(mktemp -d)
trap "rm -rf $STATUS_DIR" EXIT

i=0
VM_NAMES=()
VM_DNS=()
for vm in $vms; do
  i=$(($i+1))
  FOLLOWER_NODE="$FULL_NODE"
  if [ $NUM_FULL_NODE -gt 0 ] && [ $i -gt $NUM_FULL_NODE ]; then
    FOLLOWER_NODE="node_no_besched"
  fi
  KERNELID_PREFIX=""
  # If running with besched-only nodes, then node 0 is the leader node, the
  # following NUM_BESCHED_NODE nodes are the besched-only nodes, and the remainder
  # are nodes without bescheds.
  if [ $NUM_BESCHED_NODE -gt 0 ]; then
    if [ $i -gt $(($NUM_BESCHED_NODE + 1)) ]; then
      FOLLOWER_NODE="node_no_besched"
    else
      # If this is a besched-only follower node, prefix the kernel ID to denote
      # this so that realmd doesn't try to start per-realm services (like UX)
      # on it.
      if [ $i -gt 1 ]; then
        KERNELID_PREFIX="kernel-besched-"
      fi
      FOLLOWER_NODE="besched_node"
    fi
  fi
  if [ $i -eq 1 ]; then
    echo "starting SigmaOS on $vm nodetype leader $LEADER_NODE"
  else
    if [ $i -le $SEQ_UNTIL ]; then
      echo "starting SigmaOS on $vm nodetype follower $FOLLOWER_NODE"
    else
      echo "starting SigmaOS on $vm nodetype follower $FOLLOWER_NODE (in parallel)"
    fi
  fi
  $DIR/setup-for-benchmarking.sh $vm $TURBO
  # Get hostname.
  VM_NAME=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm hostname -s)
  KERNELID="${KERNELID_PREFIX}sigma-$VM_NAME-$(echo $RANDOM | md5sum | head -c 3)"
  VM_NAMES[$i]=$VM_NAME
  VM_DNS[$i]=$vm
  # Always a background subshell, so that the ordered nodes and the parallel
  # ones run the same code; the ordered ones are simply waited for right away.
  # The subshell captures this iteration's vm/KERNELID/FOLLOWER_NODE.
  (
  ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm <<ENDSSH
  mkdir -p /tmp/sigmaos
  export SIGMAPERF="$SIGMAPERF"
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
  # Stage this node's read-only job input before starting its kernel, which
  # bind-mounts the staging directory into the container for UX to serve.
  if [ -n "${INPUT_DATA}" ]; then
    ./download-input-data.sh ${INPUT_DATA}
  fi
  sudo ./load-apparmor.sh
  if [ "${RELOAD_GVISOR}" = "true" ]; then
    docker pull arielszekely/sigmauser:$TAG
    sudo ./create-gvisor-bundle.sh --user_ctr arielszekely/sigmauser:${TAG}
  fi

  echo "$PWD $SIGMADEBUG"
  if [ "${vm}" = "${MAIN}" ]; then
    echo "START DB: ${MAIN_PRIVADDR}"
    ./start-db.sh
    echo "DB started"
    ./make.sh --norace linux
    echo "START NETWORK $MAIN_PRIVADDR"
    ./start-network.sh --addr $MAIN_PRIVADDR
    echo "START ${SIGMASTART} ${SIGMASTART_PRIVADDR} ${KERNELID}"
    if ! docker ps | grep -q etcd ; then
      echo "START etcd"
      ./start-etcd.sh
    fi
    ./start-kernel.sh --boot $LEADER_NODE --named ${SIGMASTART_PRIVADDR} --pull ${TAG} --reserveMcpu ${RMCPU} --dbip ${MAIN_PRIVADDR}:3306 --mongoip ${MAIN_PRIVADDR}:27017 ${DIALPROXY} ${GVISOR} ${KERNELID} 2>&1 | tee /tmp/start.out
#    docker cp ~/1.jpg ${KERNELID}:/home/sigmaos/1.jpg
#    docker cp ~/6.jpg ${KERNELID}:/home/sigmaos/6.jpg
#    docker cp ~/7.jpg ${KERNELID}:/home/sigmaos/7.jpg
    docker cp ~/8.jpg ${KERNELID}:/home/sigmaos/8.jpg
  else
    echo "JOIN ${SIGMASTART} ${KERNELID}"
    ${TOKEN} 2>&1 > /dev/null
    ./start-kernel.sh --boot $FOLLOWER_NODE --named ${SIGMASTART_PRIVADDR} --pull ${TAG} --dbip ${MAIN_PRIVADDR}:3306 --mongoip ${MAIN_PRIVADDR}:27017 ${DIALPROXY} ${GVISOR} ${KERNELID} 2>&1 | tee /tmp/join.out
#    docker cp ~/1.jpg ${KERNELID}:/home/sigmaos/1.jpg
#    docker cp ~/6.jpg ${KERNELID}:/home/sigmaos/6.jpg
#    docker cp ~/7.jpg ${KERNELID}:/home/sigmaos/7.jpg
    docker cp ~/8.jpg ${KERNELID}:/home/sigmaos/8.jpg
  fi
ENDSSH
  echo $? > $STATUS_DIR/$i
  ) &
  pid=$!
  if [ $i -le $SEQ_UNTIL ]; then
    # An ordered node: wait for it before starting the next one. A failure here
    # means the rest of the cluster has nothing to join, so bail out rather than
    # starting nodes that cannot work.
    wait $pid
    if [ "$(cat $STATUS_DIR/$i 2>/dev/null)" != "0" ]; then
      echo "!!!!!!!!!! FAILED to start $VM_NAME ($vm) !!!!!!!!!!" 1>&2
      exit 1
    fi
    if [ "${vm}" = "${MAIN}" ]; then
      # The followers join the swarm with this token, so it has to be read after
      # the leader is up and before any follower starts.
      TOKEN=$(ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$vm docker swarm join-token worker | grep docker)
    fi
  fi
done
# Wait for the nodes started in parallel.
wait

nfail=0
for j in $(seq 1 $i); do
  if [ "$(cat $STATUS_DIR/$j 2>/dev/null)" != "0" ]; then
    echo "!!!!!!!!!! FAILED to start ${VM_NAMES[$j]} (${VM_DNS[$j]}) !!!!!!!!!!" 1>&2
    nfail=$(($nfail+1))
  fi
done
if [ $nfail -ne 0 ]; then
  echo "!!!!!!!!!! FAILED to start $nfail/$i node(s) !!!!!!!!!!" 1>&2
  exit 1
fi
echo "Started $i node(s)"
