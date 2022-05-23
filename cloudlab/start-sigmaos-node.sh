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

export ROOT_NAMED_ADDR=$2
export IS_LEADER=0

if [[ $ROOT_NAMED_ADDR == ":"* ]]; then
  export IS_LEADER=1
fi

# Create the ssh command string, only evaluating some variables.
CMD=$(
envsubst '$ROOT_NAMED_ADDR:$IS_LEADER:$N_REPLICAS' <<'ENDSSH'
ulimit -n 100000
export NAMED=$ROOT_NAMED_ADDR
export N_REPLICAS=$N_REPLICAS
cd ulambda

echo "running with NAMED=$NAMED"

if [[ $IS_LEADER -gt 0 ]]; then
  echo "each realm runs with $N_REPLICAS replicas"
  
  # Boot a realm
  GOGC=off nohup ./bin/realm/boot . > leader.out 2>&1 &

else
  GOGC=off nohup ./bin/realm/noded . $(hostname) > noded-$(hostname).out 2>&1 &
fi
ENDSSH
)

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
  $CMD
ENDSSH
