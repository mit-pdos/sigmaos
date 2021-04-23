#!/bin/sh

./umount.sh
killall memfsd
killall locald
killall sleeperl
killall kvd
killall coord
killall flwr
killall sharderd
killall nps3d
killall npuxd
killall fsreader
