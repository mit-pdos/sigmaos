#!/bin/bash

#
# script to set up etcd container
#

# docker pull bitnami/etcd:latest

export DATA_DIR="etcd-data"

if ! docker volume ls | grep -q $DATA_DIR; then
    echo "create vol"
    docker volume create --name ${DATA_DIR}
fi

#    --volume=${DATA_DIR}:/etcd-data \         

docker run -d \
    --name etcd-server \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    --publish 3379:2379 \
    --publish 3380:2380 \
    --publish 3381:2381 \
    --publish 3382:2382 \
    --publish 3383:2383 \
    bitnami/etcd:latest


# Or: docker container start 9bbe7bca42f0, if there is a containerid with etcd volume
