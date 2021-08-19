#!/bin/sh

# ./mk-kv-job.sh | ./bin/user/submit

L="/mnt/9p/ulambd/dev"

SPID=$((1 + $RANDOM % 1000000))
echo $SPID,"./bin/user/sharderd","bin","[]","[]",""

