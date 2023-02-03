#!/bin/bash

#
# Start container
#

usage() {
    echo "Usage: $0 [--boot all|node|named|realm] [--machine N] [--named ADDRs] [--host] "  1>&2
}

UPDATE=""
BOOT="named"
NAMED=":1111"
DBIP="x.x.x.x"
NET="bridge"

while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --machine)
    shift
    MACHINE=$1
    shift
    ;;
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
  --host)
    shift
    NET="host"
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

docker pull arielszekely/sigmauser
docker tag arielszekely/sigmauser sigmauser

if docker ps | grep -q sigmadb; then
    DBIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmadb)
fi

# Mounting docker.sock is bad idea in general because it requires to
# give rw permission on host to privileged daemon.  But maybe ok in
# our case where kernel is trusted.
CID=$(docker run -dit\
             --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock\
             --mount type=bind,src=/tmp/sigmaos,dst=/tmp/sigmaos\
             --mount type=bind,src=${HOME}/.aws,dst=/home/sigmaos/.aws\
             --network ${NET}\
             -e named=${NAMED}\
             -e boot=${BOOT}\
             -e dbip=${DBIP}\
             -e SIGMADEBUG=${SIGMADEBUG}\
             arielszekely/sigmaos)

IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${CID})
if [ -z  ${IP} ]; then
    # running with --network bridge; find out what host's IP is.
    IP=$(ip route get 8.8.8.8 | head -1 | cut -d ' ' -f 7)
    echo $IP
fi

# XXX maybe use mount to see if name is up
until [ "`docker inspect -f {{.State.Running}} ${CID}`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;
sleep 1

echo -n $IP

echo " container ${CID:0:10}" dbIP $DBIP 1>&2
