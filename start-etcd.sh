#!/bin/bash

#
# script to set up etcd container
#

# docker pull bitnami/etcd:latest

export DATA_DIR="etcd-data"

if ! docker volume ls | grep -q etcd-data; then
    echo "create vol"
    docker volume create --name ${DATA_DIR}
fi

#    --volume=${DATA_DIR}:/etcd-data \         

docker run -d \
    --name etcd-server \
    --publish 2379:2379 \
    --publish 2380:2380 \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    bitnami/etcd:latest


