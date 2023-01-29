#!/bin/bash

usage() {
  echo "Usage: $0 [--boot SRVS] [--machine N] [--named ADDRs]"  1>&2
}

UPDATE=""
BOOT="named"
NAMED=":1111"

while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --machine)
    shift
    MACHINE=$1
    shift
    ;;
  --boot)
    shift
    BOOT=$1
    shift
    ;;
  --named)
    shift
    NAMED=$1
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

# Mounting docker.sock is bad idea in general because it requires to
# give rw permission on host to privileged daemon.  But maybe ok in
# our case where kernel is trusted.
CID=$(docker run -dit\
             --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock\
             --mount type=bind,src=/tmp/sigmaos,dst=/tmp/sigmaos\
             --mount type=bind,src=${HOME}/.aws,dst=/home/sigmaos/.aws\
             -e named=${NAMED}\
             -e boot=${BOOT}\
             -e SIGMADEBUG=${SIGMADEBUG}\
             sigmaos)
IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${CID})

# XXX maybe use mount to see if name is up
until [ "`docker inspect -f {{.State.Running}} ${CID}`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;
sleep 1

echo -n $IP

echo " container ${CID:0:10}" 1>&2
