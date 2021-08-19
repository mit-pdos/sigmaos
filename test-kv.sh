#!/bin/bash

# Make sure we built without race detector
#./make.sh -norace

./start.sh

echo "start kv"

./bin/user/kvd

sleep 1

./bin/user/kvclerk

./stop.sh
