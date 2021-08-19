#!/bin/bash

# ./mk-gg-ulambda-job.sh | ./bin/user/submit

targets=$@

SPID=$((1 + $RANDOM % 1000000))
echo $SPID,"./bin/user/gg-naive-orchestrator","[name/fs/gg ${targets}]","[]","[]",""
