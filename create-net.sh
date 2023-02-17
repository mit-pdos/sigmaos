#!/bin/bash

usage() {
  echo "Usage: $0 net" 1>&2
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

if ! docker network ls | grep -q "$1"; then
    docker network create --driver overlay $1 --attachable    
fi
