#!/bin/sh

# ./mk-kv-job.sh | ./bin/submit

L="/mnt/9p/ulambd/dev"
N=1

for i in {0..$N}
do
    PID=$((1 + $RANDOM % 1000000))
    args=( "0-100" "" )
    echo $PID,"./bin/kvd","[${args[@]}]","",""
done
