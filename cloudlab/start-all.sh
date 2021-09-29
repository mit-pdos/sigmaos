#!/bin/bash

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  if [[ $hostname == $LEADER ]]; then
    $DIR/start.sh $USER@$addr :1111
    sleep 2
  else
    $DIR/start.sh $USER@$addr $LEADER:1111
  fi
done < $DIR/$SERVERS
