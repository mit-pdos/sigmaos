#!/bin/bash

usage() {
  echo "Usage: $0 --set SET --start START --end END" 1>&2
}

SET=""
START=""
END=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --set)
    shift
    SET=$1
    shift
    ;;
  --start)
    shift
    START=$1
    shift
    ;;
  --end)
    shift
    END=$1
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$SET" ] || [ -z "$START" ] || [ -z "$END" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi



for i in $(seq $START $END)
do
  echo "Setting core $i to $SET"
  echo $SET | sudo tee /sys/devices/system/cpu/cpu$i/online
done
