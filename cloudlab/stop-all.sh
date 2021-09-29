#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  $DIR/stop.sh $USER@$addr
done < $DIR/$SERVERS
