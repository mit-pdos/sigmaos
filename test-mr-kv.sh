#!/bin/bash

# Make sure we built without race detector
#./make.sh -norace

./start.sh

echo "start kv"

./bin/kvd  &

echo "start mr"

./bin/mr-wc &

sleep 1

./bin/kvclerk

echo "done"

./stop.sh
