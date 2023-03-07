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

img_name=jaegertracing/all-in-one
img_version=1.42
img=$img_name:$img_version

if ! docker ps | grep -q sigmajaeger; then
  echo "start jaeger"
  docker run -d --name sigmajaeger \
    -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
    -e COLLECTOR_OTLP_ENABLED=true \
    -p 6831:6831/udp \
    -p 6832:6832/udp \
    -p 5778:5778 \
    -p 16686:16686 \
    -p 4317:4317 \
    -p 4318:4318 \
    -p 14250:14250 \
    -p 14268:14268 \
    -p 14269:14269 \
    -p 9411:9411 \
    $img
fi

until [ "`docker inspect -f {{.State.Running}} sigmajaeger`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmajaeger)

echo "jaeger IP: $ip"
