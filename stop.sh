#!/bin/sh

./umount.sh
killall named
killall schedd
killall kvd
killall sharderd
killall nps3d
