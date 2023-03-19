#!/bin/bash

usage() {
  echo "Usage: $0" 1>&2
}

# enter swarm mode so that we can make network
if ! docker node ls | grep -q 'Leader'; then
    docker swarm init
fi 

# one network for tests
if ! docker network ls | grep -q 'sigmanet-testuser'; then
    docker network create --driver overlay sigmanet-testuser --attachable
fi

