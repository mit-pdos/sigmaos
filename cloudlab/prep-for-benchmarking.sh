#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi

DIR=$(dirname $0)

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
cd ulambda
./turn-off-hyperthread-siblings.sh
ENDSSH
