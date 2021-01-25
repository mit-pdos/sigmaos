#!/bin/bash

# ./mk-kv-job.sh | ./bin/submit

L="/mnt/9p/ulambd/dev"
N=1

GG_DIR="/mnt/9p/fs/.gg"
hashes=$@

SPID=$((1 + $RANDOM % 1000000))
echo $SPID,"gg-execute","[--timelog --ninep ${hashes[@]}]","[GG_STORAGE_URI=9p://mnt/9p/fs GG_DIR=/mnt/9p/fs/.gg]","",""
