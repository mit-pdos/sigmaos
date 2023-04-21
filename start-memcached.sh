#!/bin/bash

#
# script to set up container with jaeger.
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

img_name=memcached
img=$img_name

if ! docker ps | grep -q sigmajaeger; then
  echo "start memcached"
  docker run -d --name sigmamemcached \
    --network host \
    $img
fi

until [ "`docker inspect -f {{.State.Running}} sigmamemcached`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmamemcached)

echo "jaeger IP: $ip"
