#!/bin/bash

# Make sure we built without race detector
#./make.sh -norace

./start.sh

echo "start mr"

./bin/user/mr-wc

./stop.sh
