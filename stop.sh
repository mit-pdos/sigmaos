#!/bin/sh

./umount.sh
killall memfsd
killall memfs-replica
killall npux-replica
killall replica-monitor
killall idemproc-monitor
killall perf-memfs-replica
killall procd 
killall sleeperl
killall kv
killall monitor
killall kvd
killall kvclerk
killall coord
killall flwr
killall sharderd
killall fss3d
killall fsuxd
killall fsreader
