#!/bin/bash

if [ "$#" -ne 1 ]
then
  echo "Usage: $0 address"
  exit 1
fi
DIR=$(dirname $0)
source $DIR/env.sh

ssh -i $DIR/keys/cloudlab-sigmaos $LOGIN@$1 <<"ENDSSH"
## Set minikube memory & CPU limits
#minikube --memory 4096 --cpus 2 start

# Install hadoop node
helm install hadoop \
    --set yarn.nodeManager.resources.limits.memory=4096Mi \
    --set yarn.nodeManager.replicas=1 \
    stable/hadoop
ENDSSH
