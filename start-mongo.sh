#!/bin/bash

#
# script to set up container with mariadb in and initialize db for hotel
#

#
# to stop:
# $ docker stop sigmadb
#
# to remove
# $ docker rm -v sigmadb
#

usage() {
  echo "Usage: $0 yml..."
}

PORT=4406  # use non-default port number on host

docker pull mongo:4.4.6
if ! docker ps | grep -q sigmamongo; then
    echo "start mongodb"
    docker run --name sigmamongo -d mongo:4.4.6
fi

until [ "`docker inspect -f {{.State.Running}} sigmamongo`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmamongo)

echo "mongo IP: $ip"

