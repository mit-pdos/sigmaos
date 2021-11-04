#!/bin/bash

DIR=$(dirname $0)

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<ENDSSH
cd ulambda
./turn-off-hyperthread-siblings.sh
ENDSSH
