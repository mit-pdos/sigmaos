#!/bin/bash

# ./mk-gg-ulambda-job.sh | ./bin/user/submit

targets=$@

SPID=$((1 + $RANDOM % 1000000))
echo $SPID,"./bin/user/gg-orchestrator","[name/fs/gg ${targets}]","[]","[]",""
