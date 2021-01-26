#!/bin/bash

# ./mk-gg-ulambda-job.sh | ./bin/submit

targets=$@

SPID=$((1 + $RANDOM % 1000000))
echo $SPID,"./bin/gg","[${targets}]","[]","[]",""
