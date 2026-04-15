#!/bin/bash

#
# Start kernel container
#

set -x

usage() {
    echo "Usage: $0 [--pull TAG] [--boot all|all_no_besched|node|node_no_besched|minnode|besched_node|named|realm_no_besched|spproxyd] [--named ADDRs] [--dbip DBIP] [--mongoip MONGOIP] [--usedialproxy] [--usegvisor] [--reserveMcpu rmcpu] [--homedir HOMEDIR] [--projectroot PROJECT_ROOT] [--sigmauser SIGMAUSER] [--net NETNAME] kernelid"  1>&2
}

UPDATE=""
TAG="local-build"
BOOT="named"
NAMED="127.0.0.1"
DBIP="x.x.x.x"
MONGOIP="x.x.x.x"
NET="host"
KERNELID=""
DIALPROXY="false"
GVISOR="false"
RMCPU="0"
HOMEDIR=$HOME
PROJECT_ROOT=$(realpath $(dirname $0))
SIGMAUSER="NOT_SET"

while [[ "$#" -gt 1 ]]; do
  case "$1" in
  --boot)
    shift
    case "$1" in
        "all")
            BOOT="knamed;besched;lcsched;msched;ux;s3;chunkd;pyenv;db;mongo"
            ;;
        "all_no_besched")
            BOOT="knamed;lcsched;msched;ux;s3;chunkd;pyenv;db;mongo"
            ;;
        "node")
            BOOT="besched;msched;ux;s3;db;chunkd;pyenv;mongo"
            ;;
        "node_no_besched")
            BOOT="msched;ux;s3;db;chunkd;pyenv;mongo"
            ;;
        "minnode")
            BOOT="msched;ux;s3;chunkd;pyenv"
            ;;
        "besched_node")
            BOOT="besched"
            ;;
        "named")
            BOOT="knamed"
            ;;
        "spproxyd")
            BOOT="spproxyd"
            ;;
        "realm")
            BOOT="knamed;besched;lcsched;msched;realmd;ux;s3;chunkd;pyenv;db;mongo"
            ;;
        "realm_no_besched")
            BOOT="knamed;lcsched;msched;realmd;ux;s3;chunkd;pyenv;db;mongo"
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
  --net)
    shift
    NET=$1
    shift
    ;;
  --sigmauser)
    shift
    SIGMAUSER=$1
    shift
    ;;
  --usedialproxy)
    shift
    DIALPROXY="true"
    ;;
  --usegvisor)
    shift
    GVISOR="true"
    ;;
  --named)
    shift
    NAMED=$1
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
  --homedir)
    shift
    HOMEDIR=$1
    shift
    ;;
  --projectroot)
    shift
    PROJECT_ROOT=$1
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

KERNEL_IMAGE_NAME="sigmaos"
DB_IMAGE_NAME="sigmadb"
MONGO_IMAGE_NAME="sigmamongo"
TMP_BASE="/tmp"
if [[ "$SIGMAUSER" != "NOT_SET" ]]; then
  TMP_BASE=$TMP_BASE/$SIGMAUSER
  KERNEL_IMAGE_NAME=$KERNEL_IMAGE_NAME-$SIGMAUSER
  DB_IMAGE_NAME=$DB_IMAGE_NAME-$SIGMAUSER
  MONGO_IMAGE_NAME=$MONGO_IMAGE_NAME-$SIGMAUSER
fi

HOST_BIN_CACHE="${TMP_BASE}/sigmaos-bin"
DATA_DIR="${TMP_BASE}/sigmaos-data"
PERF_DIR="${TMP_BASE}/sigmaos-perf"
KERNEL_DIR="${TMP_BASE}/sigmaos"
SPPROXY_DIR="${TMP_BASE}/spproxyd"
PYTHON_PKG_CACHE_DIR="${TMP_BASE}/sigmaos-python"

mkdir -p $SPPROXY_DIR
mkdir -p $HOST_BIN_CACHE
mkdir -p $HOST_BIN_CACHE/$KERNELID
mkdir -p $DATA_DIR
mkdir -p $PERF_DIR
chmod a+w $PERF_DIR
mkdir -p $KERNEL_DIR
mkdir -p $PYTHON_PKG_CACHE_DIR
mkdir -p $PYTHON_PKG_CACHE_DIR/$KERNELID

# Pull latest docker images, if not running a local build.
if [ "$TAG" != "local-build" ]; then
  docker pull arielszekely/sigmaos:$TAG > /dev/null
  docker tag arielszekely/sigmaos:$TAG sigmaos > /dev/null
  docker pull arielszekely/sigmauser:$TAG > /dev/null
  docker tag arielszekely/sigmauser:$TAG sigmauser > /dev/null
fi

if [ "$NAMED" == ":1111" ] && ! docker ps | grep -q etcd ; then
  ./start-etcd.sh
fi

if [ "$DBIP" == "x.x.x.x" ] && docker ps | grep -q $DB_IMAGE_NAME; then
  DBIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $DB_IMAGE_NAME):3306
fi

if [ "$MONGOIP" == "x.x.x.x" ] && docker ps | grep -q $MONGO_IMAGE_NAME; then
  MONGOIP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $MONGO_IMAGE_NAME):27017
fi

# Mounting docker.sock is bad idea in general because it requires to
# give rw permission on host to privileged daemon.  But maybe ok in
# our case where kernel is trusted.
MOUNTS="--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
  --mount type=bind,src=/sys/fs/cgroup,dst=/cgroup \
  --mount type=bind,src=$KERNEL_DIR,dst=/tmp/sigmaos \
  --mount type=bind,src=$SPPROXY_DIR,dst=/tmp/spproxyd \
  --mount type=bind,src=$DATA_DIR,dst=/home/sigmaos/data \
  --mount type=bind,src=$HOST_BIN_CACHE/${KERNELID},dst=/home/sigmaos/bin/user/realms \
  --mount type=bind,src=$PYTHON_PKG_CACHE_DIR/${KERNELID},dst=/tmp/python/package-cache \
  --mount type=bind,src=$PERF_DIR,dst=/tmp/sigmaos-perf \
  --mount type=bind,src=$HOMEDIR/.aws,dst=/home/sigmaos/.aws"
# If running in local configuration, mount bin directory.
if [ "$TAG" == "local-build" ]; then
  MOUNTS="$MOUNTS\
    --mount type=bind,src=$PROJECT_ROOT/bin/user,dst=/home/sigmaos/bin/user/common,ro \
    --mount type=bind,src=$PROJECT_ROOT/bin/kernel,dst=/home/sigmaos/bin/kernel,ro \
    --mount type=bind,src=$PROJECT_ROOT/bin/linux,dst=/home/sigmaos/bin/linux,ro"
fi

CID=$(docker run -dit \
             $MOUNTS \
             --pid host \
             --privileged \
             --network ${NET} \
             --name ${KERNELID} \
             -e kernelid=${KERNELID} \
             -e named=${NAMED} \
             -e boot=${BOOT} \
             -e dbip=${DBIP} \
             -e mongoip=${MONGOIP} \
             -e buildtag=${TAG} \
             -e dialproxy=${DIALPROXY} \
             -e gvisor=${GVISOR} \
             -e SIGMAPERF=${SIGMAPERF} \
             -e SIGMAFAIL=${SIGMAFAIL} \
             -e SIGMADEBUG=${SIGMADEBUG} \
             -e SIGMAVALGRIND=${SIGMAVALGRIND} \
             -e SIGMADEBUGPROCS=${SIGMADEBUGPROCS} \
             -e SIGMAPYMGRMODE=${SIGMAPYMGRMODE} \
             -e reserveMcpu=${RMCPU} \
             -e netmode=${NET} \
             -e sigmauser=${SIGMAUSER} \
             $KERNEL_IMAGE_NAME)

if [ -z ${CID} ]; then
    echo "Docker run failed $?"  1>&2
    exit 1
fi

IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${CID})
if [ "${IP}" == "invalid IP" ]; then
  IP=""
fi
if [ -z  "${IP}" ]; then
    # find out what host's IP is (e.g., when running with --network bridge)
    IP=$(ip route get 8.8.8.8 | head -1 | cut -d ' ' -f 7)
fi

until [ "`docker inspect -f {{.State.Running}} ${CID}`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

# Wait until kernel is ready
while [ ! -f "${KERNEL_DIR}/${KERNELID}" ]; do
    echo -n "." 1>&2
    sleep 0.1
done;
rm -f "${KERNEL_DIR}/${KERNELID}"

echo "nproc: $(nproc)"
echo "booted: $BOOT"

echo -n $IP

echo " container ${CID:0:10}" dbIP $DBIP  mongoIP $MONGOIP 1>&2
