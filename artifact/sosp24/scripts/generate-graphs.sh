#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/artifact/sosp24/scripts

$SCRIPTS_DIR/generate-aws-graphs.sh
$SCRIPTS_DIR/generate-cloudlab-graphs.sh
