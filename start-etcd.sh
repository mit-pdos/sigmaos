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
    --publish 2381:2381 \
    --publish 2382:2382 \
    --publish 2383:2383 \
    -listen-client-urls http://localhost:2379,http://localhost:2380,http://localhost:2381,http://localhost:2382, http://localhost:2383 \
    --env ALLOW_NONE_AUTHENTICATION=yes \
    bitnami/etcd:latest


# Or: docker container start 9bbe7bca42f0, if there is a containerid with etcd volume
