#!/bin/bash

usage() {
  echo "Usage: $0 net" 1>&2
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

if ! docker network ls | grep -q "sigmanet-$1"; then
    docker network create --driver overlay sigmanet-$1 --attachable    
fi
