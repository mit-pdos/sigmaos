#!/bin/bash

usage() {
  echo "Usage: $0 [--machine N]"  1>&2
}

UPDATE=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --machine)
    shift
    MACHINE=$1
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

mkdir -p /tmp/sigmaos

# default arguments to bootkernel
SIGMANAMED=":1111"
SIGMABOOT="named"

echo "running with SIGMANAMED=$SIGMANAMED"

docker run -dit --mount type=bind,src=/tmp/sigmaos,dst=/tmp/sigmaos -e named=${SIGMANAMED} -e boot=${SIGMABOOT} -e SIGMADEBUG=${SIGMADEBUG} sigmaos
 
sleep 1

