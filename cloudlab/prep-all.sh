#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

ssh -i $DIR/keys/cloudlab-sigmaos $USER@$addr <<ENDSSH
cd ulambda
./turn-off-hyperthread-siblings.sh
ENDSSH

done < $DIR/$SERVERS
