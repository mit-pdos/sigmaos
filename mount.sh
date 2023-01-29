#!/bin/sh

echo $1

./bin/linux/proxyd $1 $1:1111 &

sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p
