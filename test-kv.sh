#!/bin/bash

# Make sure we built without race detector
#./make.sh -norace

./start.sh

echo "start kv"

./bin/kvd

sleep 1

./bin/kvclerk

./stop.sh
