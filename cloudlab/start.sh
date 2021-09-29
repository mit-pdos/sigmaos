#!/bin/bash

if [ "$#" -ne 2 ]
then
  echo "Usage: ./install-sw.sh user@address root_named_addr"
  exit 1
fi

export ROOT_NAMED_ADDR=$2
export IS_LEADER=0

if [[ $ROOT_NAMED_ADDR == ":"* ]]; then
  export IS_LEADER=1
fi

# Create the ssh command string, only evaluating some variables.
CMD=$(
envsubst '$ROOT_NAMED_ADDR:$IS_LEADER' <<'ENDSSH'
export NAMED=$ROOT_NAMED_ADDR
cd ulambda

if [[ $IS_LEADER -gt 0 ]]; then
  nohup ./start.sh &
else
  nohup ./bin/realm/realmd . $(hostname) &
fi
ENDSSH
)

ssh $1 <<ENDSSH
  $CMD
ENDSSH
