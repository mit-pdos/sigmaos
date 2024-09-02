#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/artifact/sosp24/scripts

$SCRIPTS_DIR/run-aws-experiments.sh
$SCRIPTS_DIR/run-cloudlab-experiments.sh
