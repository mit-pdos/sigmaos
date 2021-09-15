#!/bin/sh

./umount.sh
killall realmmgr
killall realmd
killall create
killall destroy
killall named
killall memfsd
killall memfs-chain-replica
killall memfs-raft-replica
killall fsux-chain-replica
killall replica-monitor
killall procd-monitor
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
killall wwwd
