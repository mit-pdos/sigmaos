#!/bin/bash

#
# script to set up etcd container
#

# docker pull bitnami/etcd:latest

ROOT=$(dirname $(realpath $0))
source $ROOT/env/env.sh

ETCD_CTR_NAME="etcd-server"
NETWORK="host"
if ! [ -z "$SIGMAUSER" ]; then
  ETCD_CTR_NAME="etcd-tester-${SIGMAUSER}"
  NETWORK="sigmanet-testuser-${SIGMAUSER}"
fi
DATA_DIR="$ETCD_CTR_NAME-data"

if ! docker volume ls | grep -q $DATA_DIR; then
  echo "create vol"
  docker volume create --name $DATA_DIR
fi

if ! [ -z "$SIGMAUSER" ]; then
  docker run --rm -d \
    --name $ETCD_CTR_NAME \
    --network $NETWORK \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    arielszekely/etcd:latest
else
  docker run --rm -d \
    --name $ETCD_CTR_NAME \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    --network host \
    arielszekely/etcd:latest
#    --publish 3379:2379 \
#    --publish 3380:2380 \
#    --publish 3381:2381 \
#    --publish 3382:2382 \
#    --publish 3383:2383 \
fi
