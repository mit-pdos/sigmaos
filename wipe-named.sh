#!/bin/bash

# Delete all keys in named

docker exec etcd-server etcdctl del --prefix ''
