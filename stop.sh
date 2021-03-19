#!/bin/sh

./umount.sh
killall named
killall schedd
killall locald
killall schedl
killall kvd
killall sharderd
killall nps3d
killall npuxd
