#!/bin/bash

# Set minikube memory & CPU limits
minikube --memory 4096 --cpus 2 start

# Install hadoop node
helm install hadoop \
    --set yarn.nodeManager.resources.limits.memory=4096Mi \
    --set yarn.nodeManager.replicas=1 \
    stable/hadoop

