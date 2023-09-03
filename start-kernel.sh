#!/bin/bash

#
# Start kernel container
#

usage() {
    echo "Usage: $0 [--pull TAG] [--boot all|node|named|realm] [--named ADDRs] [--dbip DBIP] [--mongoip MONGOIP] [--jaeger JAEGERIP] [--host] [--overlays] [--reserveMcpu rmcpu] kernelid"  1>&2
}

UPDATE=""
TAG=""
BOOT="named"
NAMED=":1111"
DBIP="x.x.x.x"
MONGOIP="x.x.x.x"
JAEGERIP="$(hostname -i | cut -f 1 -d ' ')"
NET="host"
KERNELID=""
OVERLAYS="false"
RMCPU="0"
while [[ "$#" -gt 1 ]]; do
  case "$1" in
  --boot)
    shift
    case "$1" in
        "all")
            BOOT="knamed;schedd;ux;s3;db;mongo;named"
            ;;
        "node")
            BOOT="schedd;ux;s3;db;mongo"
            ;;
        "named")
            BOOT="knamed"
            ;;
        "realm")
            BOOT="knamed;schedd;realmd;ux;s3;db;mongo;named"
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
  --jaeger)
    shift
    JAEGERIP=$1
    shift
    ;;
  --dbip)
    shift
    DBIP=$1
    shift
    ;;
  --mongoip)
    shift
    MONGOIP=$1
    shift
    ;;
  --reserveMcpu)
    shift
    RMCPU=$1
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

if [ "$NAMED" == ":1111" ] && ! docker ps | grep -q etcd ; then
  ./start-etcd.sh
fi

if [ "$DBIP" == "x.x.x.x" ] && docker ps | grep -q sigmadb; then
  DBIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmadb):3306
fi

if [ "$MONGOIP" == "x.x.x.x" ] && docker ps | grep -q sigmamongo; then
  MONGOIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmamongo):27017
fi

# Mounting docker.sock is bad idea in general because it requires to
# give rw permission on host to privileged daemon.  But maybe ok in
# our case where kernel is trusted.
CID=$(docker run -dit\
             --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock\
             --mount type=bind,src=/sys/fs/cgroup,dst=/cgroup\
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
             -e mongoip=${MONGOIP}\
             -e jaegerip=${JAEGERIP}\
             -e overlays=${OVERLAYS}\
             -e SIGMADEBUG=${SIGMADEBUG}\
             -e SIGMANAMED=${SIGMANAMED}\
             -e reserveMcpu=${RMCPU}\
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

until [ "`docker inspect -f {{.State.Running}} ${CID}`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

# Wait until kernel is ready
while [ ! -f "/tmp/sigmaos/${KERNELID}" ]; do
    echo -n "." 1>&2
    sleep 0.1
done;
rm -f "/tmp/sigmaos/${KERNELID}"

echo -n $IP

echo " container ${CID:0:10}" dbIP $DBIP  mongoIP $MONGOIP 1>&2
