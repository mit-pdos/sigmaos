#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: ./run-script-all.sh script_path"
  exit 1
fi

SCRIPT_PATH=$1

DIR=$(dirname $0)

. $DIR/config

while read -r line
do
  tuple=($line)

  hostname=${tuple[0]}
  addr=${tuple[1]}

  $SCRIPT_PATH $USER@$addr
done < $DIR/$SERVERS
