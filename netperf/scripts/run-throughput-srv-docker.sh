#!/bin/bash

# Stop any prior running server
if docker ps -a | grep -qE 'sigmaos-netperf'; then
  for container in $(docker ps -a | grep -E 'sigmaos-netperf' | cut -d ' ' -f1) ; do
      docker stop $container 
  done
fi

# Start the server container
CID=$(docker run \
  --net=sigmaos-overlay-perf-test --rm -dit \
  arielszekely/sigmaos-netperf \
  go test -v netperf --run TestSrvThroughputTCP \
  --srvaddr $1 --ntrial 500
)

# Scrape its IP
IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${CID})
echo $IP

# Re-attach to the server container
docker attach $CID > /dev/null 2>&1 
