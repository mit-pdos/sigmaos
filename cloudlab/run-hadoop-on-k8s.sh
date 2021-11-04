#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 user@address"
  exit 1
fi
DIR=$(dirname $0)

. $DIR/config

ssh -i $DIR/keys/cloudlab-sigmaos $1 <<"ENDSSH"

# Set minikube memory & CPU limits
minikube --memory 4096 --cpus 2 start

# Install hadoop node
helm install hadoop \
    --set yarn.nodeManager.resources.limits.memory=4096Mi \
    --set yarn.nodeManager.replicas=1 \
    stable/hadoop

ENDSSH
