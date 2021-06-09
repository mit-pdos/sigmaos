#!/bin/sh

./umount.sh
killall memfsd
killall memfs-replica
killall npux-replica
killall replica-monitor
killall perf-memfs-replica
killall locald
killall sleeperl
killall kv
killall coord
killall flwr
killall sharderd
killall nps3d
killall npuxd
killall fsreader
