#!/bin/bash

#
# Start kernel container
#

usage() {
    echo "Usage: $0 [--pull TAG] [--boot all|node|named|realm] [--named ADDRs] [--host] [--overlays] kernelid"  1>&2
}

UPDATE=""
TAG=""
BOOT="named"
NAMED=":1111"
DBIP="x.x.x.x"
JAEGERIP="172.17.0.3"
NET="host"
KERNELID=""
OVERLAYS="false"
while [[ "$#" -gt 1 ]]; do
  case "$1" in
  --boot)
    shift
    case "$1" in
        "all")
            BOOT="named;schedd;ux;s3;db"
            ;;
        "node")
            BOOT="schedd;ux;s3;db"
            ;;
        "named")
            BOOT="named"
            ;;
        "realm")
            BOOT="named;schedd;realmd;ux;s3;db"
            ;;
        *)
            echo "unexpected argument $1 to boot"
            usage
            exit 1
            ;;
    esac            
    shift
    ;;
  --pull)
    shift
    TAG=$1
    shift
    ;;
  --host)
    shift
    NET="host"
    ;;
  --overlays)
    shift
    OVERLAYS="true"
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

if [ $# -ne 1 ]; then
    usage
    exit 1
fi
KERNELID=$1

mkdir -p /tmp/sigmaos
mkdir -p /tmp/sigmaos-bin
mkdir -p /tmp/sigmaos-perf

# Pull latest docker images
if ! [ -z "$TAG" ]; then
  docker pull arielszekely/sigmaos:$TAG > /dev/null
  docker tag arielszekely/sigmaos:$TAG sigmaos > /dev/null
  docker pull arielszekely/sigmauser:$TAG > /dev/null
  docker tag arielszekely/sigmauser:$TAG sigmauser > /dev/null
fi

if docker ps | grep -q sigmadb; then
    DBIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmadb)
fi

if docker ps | grep -q sigmajaeger; then
    JAEGERIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmajaeger)
fi

# Mounting docker.sock is bad idea in general because it requires to
# give rw permission on host to privileged daemon.  But maybe ok in
# our case where kernel is trusted.
CID=$(docker run -dit\
             --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock\
             --mount type=bind,src=/tmp/sigmaos,dst=/tmp/sigmaos\
             --mount type=bind,src=/tmp/sigmaos-bin,dst=/home/sigmaos/bin/user/realms\
             --mount type=bind,src=/tmp/sigmaos-perf,dst=/tmp/sigmaos-perf\
             --mount type=bind,src=${HOME}/.aws,dst=/home/sigmaos/.aws\
             --network ${NET}\
             --name ${KERNELID}\
             -e kernelid=${KERNELID}\
             -e named=${NAMED}\
             -e boot=${BOOT}\
             -e dbip=${DBIP}\
             -e jaegerip=${JAEGERIP}\
             -e overlays=${OVERLAYS}\
             -e SIGMADEBUG=${SIGMADEBUG}\
             sigmaos)

if [ -z ${CID} ]; then
    echo "Docker run failed $?"  1>&2
    exit 1
fi

IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${CID})
if [ -z  ${IP} ]; then
    # find out what host's IP is (e.g., when running with --network bridge)
    IP=$(ip route get 8.8.8.8 | head -1 | cut -d ' ' -f 7)
fi

# XXX maybe use mount to see if name is up
until [ "`docker inspect -f {{.State.Running}} ${CID}`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;
sleep 1

echo -n $IP

echo " container ${CID:0:10}" dbIP $DBIP 1>&2
