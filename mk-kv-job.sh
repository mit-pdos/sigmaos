#!/bin/sh

# ./mk-kv-job.sh | ./bin/submit

L="/mnt/9p/ulambd/dev"
N=1

SPID=$((1 + $RANDOM % 1000000))
PID=$((1 + $RANDOM % 1000000))
pairs=("(${SPID};${PID})")
echo $SPID,"./bin/sharderd","","[]","[${pairs[@]}]",""

for i in {0..0}
do
    pairs=("(${SPID};${PID})")
    echo $PID,"./bin/kvd","[$i]","[]","[${pairs[@]}]",""
done
