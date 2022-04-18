#!/bin/bash

./bin/kernel/named :1111 no-realm &

echo "=== RUN Proxy"

sleep 1

# SIGMADEBUG="NETSRV;" ./bin/kernel/proxyd &
./bin/kernel/proxyd &

sleep 1

sudo mount -t 9p -o tcp,name=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p

