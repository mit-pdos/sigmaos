#!/bin/bash

usage() {
  echo "Usage: $0 [--addr ADVERTISE_ADDR]" 1>&2
}

ADDR=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --addr)
    shift
    ADDR="--advertise-addr $1"
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

# enter swarm mode so that we can make network
if ! docker node ls | grep -q 'Leader'; then
    docker swarm init $ADDR
fi 

# one network for tests
if ! docker network ls | grep -q 'sigmanet-testuser'; then
    docker network create --driver overlay sigmanet-testuser --attachable
fi

