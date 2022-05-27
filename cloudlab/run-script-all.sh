#!/bin/bash

if [ "$#" -lt 1 ]
then
  echo "Usage: $0 script_path [-parallel]"
  exit 1
fi

PARALLEL=0
if [ $# -gt 1 ] && [ $2 == "-parallel" ]
then
  PARALLEL=1
fi

SCRIPT_PATH=$1

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  echo "========================= $SCRIPT_PATH $USER@$addr ========================="

  if [ "$PARALLEL" -eq 1 ]
  then
    $SCRIPT_PATH $USER@$addr > $DIR/log/$hostname &
  else
    $SCRIPT_PATH $USER@$addr > $DIR/log/$hostname
  fi
done < $DIR/$SERVERS
wait
