#!/bin/bash

DIR=$(dirname $0)
source $DIR/env.sh

echo "Setting up servers"
cat $DIR/servers.txt

for s in $(cat $DIR/servers.txt | cut -d " " -f 2); do
  ./setup-instance.sh $s > /tmp/$s-instance-setup.out 2>&1 &
  sleep 10
done

wait

echo "Configuring kernels"

for s in $(cat $DIR/servers.txt | cut -d " " -f 2); do
  ./configure-kernel.sh $s > /tmp/$s-kernel-config.out 2>&1 &
  sleep 10
done

wait

echo "Done setting up cluster"