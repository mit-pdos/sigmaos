#!/bin/bash

./start.sh

GOGC=off ./bin/user/microbenchmarks

./stop.sh
