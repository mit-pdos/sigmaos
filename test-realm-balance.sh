#!/bin/bash

./stop.sh

./start.sh

# Start an extra machined
./bin/realm/machined . $(hostname)-1 &

./bin/user/realm-balance
