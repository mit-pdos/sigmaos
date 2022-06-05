#!/bin/sh

./start.sh --realm 1000

ls -a /mnt/9p/ | grep ".statsd" > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

cat /mnt/9p/.statsd | grep Nwalk > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

echo hello > /mnt/9p/xxx
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

rm /mnt/9p/xxx
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

mkdir /mnt/9p/d
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

echo hello > /mnt/9p/d/xxx
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

grep hello /mnt/9p/d/xxx > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

ls /mnt/9p/d | grep xxx > /dev/null
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

rm /mnt/9p/d/xxx
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

rmdir /mnt/9p/d
if [ $? -eq 0 ]; then
   echo OK
else
   echo FAIL
fi

ls /mnt/9p/xxx > /dev/null 2>&1
if [ $? -eq 0 ]; then
   echo "--- FAIL Proxy"
else
   echo "--- PASS: Proxy"
fi

./stop.sh

