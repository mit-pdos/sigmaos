#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/artifact/sosp24/scripts

$SCRIPTS_DIR/stop-sigmaos-aws.sh
$SCRIPTS_DIR/stop-sigmaos-cloudlab.sh
