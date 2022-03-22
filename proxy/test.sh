#!/bin/sh

./start.sh

ls -a /mnt/9p/ | grep ".statsd" > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

cat /mnt/9p/.statsd | grep Nwalk > /dev/null
if [ $? -eq 0 ]; then
   echo "--- PASS: Proxy"
else
   echo "--- FAIL Proxy"
fi

./stop.sh

