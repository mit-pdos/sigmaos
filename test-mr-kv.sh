#!/bin/bash

# Make sure we built without race detector
#./make.sh -norace

./start.sh

echo "start kv"

./bin/user/kvd  &

echo "start mr"

./bin/user/mr-wc-test &

sleep 1

./bin/user/kvclerk

echo "done"

./stop.sh
