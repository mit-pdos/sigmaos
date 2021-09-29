#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: ./install-sw.sh user@address"
  exit 1
fi

ROOT_NAMED_ADDR=$2

ssh $1 <<ENDSSH
cd ulambda
./stop.sh
ENDSSH
