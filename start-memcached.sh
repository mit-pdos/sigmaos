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

conts=$(docker ps -a -q --filter="name=mcd")
docker stop $conts
docker rm $conts

if ! docker ps | grep -q sigmajaeger; then
  echo "start memcached"
  docker run -d --name mcd \
    --network host \
    $img \
    -c 8192 -m 4096 -t 4
fi

until [ "`docker inspect -f {{.State.Running}} mcd`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' mcd)

echo "jaeger IP: $ip"
