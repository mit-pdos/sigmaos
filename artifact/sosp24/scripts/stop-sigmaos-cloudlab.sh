#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/cloudlab

cd $SCRIPTS_DIR
./stop-sigmaos.sh --parallel
