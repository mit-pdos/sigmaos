#!/bin/bash

#
# script to set up container with mariadb in and initialize db for hotel
#

export NODE1=192.168.2.114
export DATA_DIR="etcd-data"

if ! docker volume ls | grep -q etcd-data; then
    echo "create vol"
    docker volume create --name ${DATA_DIR}
fi

REGISTRY=quay.io/coreos/etcd
# available from v3.2.5
#REGISTRY=gcr.io/etcd-development/etcd

docker run \
  -p 2379:2379 \
  -p 2380:2380 \
  --volume=${DATA_DIR}:/etcd-data \
  --name etcd ${REGISTRY}:latest \
  /usr/local/bin/etcd \
  --data-dir=/etcd-data --name node1 \
  --initial-advertise-peer-urls http://${NODE1}:2380 --listen-peer-urls http://0.0.0.0:2380 \
  --advertise-client-urls http://${NODE1}:2379 --listen-client-urls http://0.0.0.0:2379 \
  --initial-cluster node1=http://${NODE1}:2380

# docker exec etcd /usr/local/bin/etcdctl  member list
