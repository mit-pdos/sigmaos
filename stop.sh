#!/bin/sh

killall named
killall proxyd
killall schedd
killall kvd
killall sharderd
sudo umount /mnt/9p
