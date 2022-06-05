#!/bin/bash

DIR=$(dirname $0)
. $DIR/.env

if [[ -z "${NAMED}" ]]; then
  export NAMED=":1111"
fi

if [[ -z "${N_REPLICAS}" ]]; then
  export N_REPLICAS=1
fi

echo "running with NAMED=$NAMED and N_REPLICAS=$N_REPLICAS"

$BIN/realm/boot

./mount.sh
