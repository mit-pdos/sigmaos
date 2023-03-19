#!/bin/bash

DIR=$(dirname $0)

if [[ -z "${N_REPLICAS}" ]]; then
  echo "Must specify number of replicas"
  exit 1
fi

if [ "$#" -ne 2 ]
then
  echo "Usage: $0 user@address root_named_addr"
  exit 1
fi

export ROOT_SIGMANAMED_ADDR=$2
export IS_LEADER=0

if [[ $ROOT_SIGMANAMED_ADDR == ":"* ]]; then
  export IS_LEADER=1
fi

# Create the ssh command string, only evaluating some variables.
CMD=$(
envsubst '$ROOT_SIGMANAMED_ADDR:$IS_LEADER:$N_REPLICAS' <<'ENDSSH'
ulimit -n 100000
export SIGMANAMED=$ROOT_SIGMANAMED_ADDR
export N_REPLICAS=$N_REPLICAS
cd ulambda

echo "running with SIGMANAMED=$SIGMANAMED"

if [[ $IS_LEADER -gt 0 ]]; then
  echo "each realm runs with $N_REPLICAS replicas"
  
  # Boot a realm  XXX broken
  ./start-kernel.sh --realm arielck > leader.out 2>&1 &

else
  SIGMAPID=machined-$HOSTNAME nohup /tmp/sigmaos/bin/realm/machined > noded-$(hostname).out 2>&1 &
fi
ENDSSH
)

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
  $CMD
ENDSSH
