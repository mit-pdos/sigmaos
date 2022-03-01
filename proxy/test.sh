#!/bin/sh

./bin/kernel/named :1111 no-realm &

echo "=== RUN PROXY"

sleep 1

./bin/kernel/proxyd &

sleep 1

sudo mount -t 9p -o tcp,name=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p

ls -l /mnt/9p/ | grep statsd > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

cat /mnt/9p/statsd | grep Nwalk > /dev/null
if [ $? -eq 0 ]; then
   echo "--- PASS: PROXY"
else
   echo "--- FAIL PROXY"
fi

./stop.sh

