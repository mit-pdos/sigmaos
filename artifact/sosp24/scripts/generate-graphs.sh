#!/bin/bash

VERSION=SOSP24AE
ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/artifact/sosp24/scripts

$SCRIPTS_DIR/generate-aws-graphs.sh
$SCRIPTS_DIR/generaet-cloudlab-graphs.sh
