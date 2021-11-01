#!/bin/bash

./start.sh

# Start an extra realmd
./bin/realm/realmd . $(hostname)-1 &

./bin/user/realm-balance

./stop.sh
