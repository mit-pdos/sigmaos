#!/bin/sh

./bin/kernel/proxyd &
sleep 1
sudo mount -t 9p -o trans=tcp,aname=`whoami`,uname=`whoami`,port=1110 127.0.0.1 /mnt/9p
