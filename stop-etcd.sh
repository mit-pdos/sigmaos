#!/bin/bash

# If etcd is up and running...
if docker ps -a | grep -qE 'bitnami/etcd'; then
  cid=$(docker ps -a | grep "bitnami/etcd" | cut -d " " -f1)
  docker stop $cid
  docker rm $cid
fi
