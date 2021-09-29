#!/bin/bash

DIR=$(dirname $0)

if [ "$#" -ne 2 ]
then
  echo "Usage: ./start.sh user@address root_named_addr"
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
export NAMED=$ROOT_NAMED_ADDR
export N_REPLICAS=$N_REPLICAS
cd ulambda

if [[ $IS_LEADER -gt 0 ]]; then
  echo "running with NAMED=$NAMED"
  echo "each realm runs with $N_REPLICAS replicas"
  
  # Start a realm manager, realmd, and create a realm
  nohup ./bin/realm/realmmgr . > realmmgr.out 2>&1 & 
  sleep 2
  nohup ./bin/realm/realmd . $(hostname) > realmd.out 2>&1 &
  sleep 1
  nohup ./bin/realm/create 1000 > create.out 2>&1 &

else
  nohup ./bin/realm/realmd . $(hostname) > realmd.out 2>&1 &
fi
ENDSSH
)

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
  $CMD
ENDSSH
