#!/bin/sh

./umount.sh
killall memfsd
killall memfs-replica
killall npux-replica
killall replica-monitor
killall perf-memfs-replica
killall procd 
killall sleeperl
killall kv
killall kvd
killall kvclerk
killall coord
killall flwr
killall sharderd
killall nps3d
killall npuxd
killall fsreader
