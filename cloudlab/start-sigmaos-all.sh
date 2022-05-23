#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  if [[ $hostname == $LEADER ]]; then
    $DIR/start-sigmaos-node.sh $USER@$addr :1111 > $DIR/log/$hostname
    sleep 2
  else
    $DIR/start-sigmaos-node.sh $USER@$addr $LEADER:1111 > $DIR/log/$hostname
  fi
done < $DIR/$SERVERS 
