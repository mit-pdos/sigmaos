#!/bin/bash

./start.sh

# Start an extra machined
./bin/realm/machined . $(hostname)-1 &

./bin/user/realm-balance

./stop.sh
