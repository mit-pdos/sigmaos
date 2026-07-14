#!/bin/bash

# If etcd is up and running...
if docker ps -a | grep -qE 'arielszekely/etcd'; then
  cid=$(docker ps -a | grep "arielszekely/etcd" | cut -d " " -f1)
  docker stop $cid
  docker rm $cid
fi
