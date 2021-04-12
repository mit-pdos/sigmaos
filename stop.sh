#!/bin/sh

./umount.sh
killall named
killall locald
killall sleeperl
killall kvd
killall sharderd
killall nps3d
killall npuxd
killall fsreader
