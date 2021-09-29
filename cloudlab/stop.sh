#!/bin/bash

DIR=$(dirname $0)

if [ "$#" -ne 1 ]
then
  echo "Usage: ./stop.sh user@address"
  exit 1
fi

ROOT_NAMED_ADDR=$2

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
cd ulambda
./stop.sh
ENDSSH
