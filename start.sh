#!/bin/bash

usage() {
  echo "Usage: $0 --realm REALM" 1>&2
}

REALM=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --realm)
    shift
    REALM=$1
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/.env

if [[ -z "${NAMED}" ]]; then
  export NAMED=":1111"
fi

if [[ -z "${N_REPLICAS}" ]]; then
  export N_REPLICAS=1
fi

echo "running with NAMED=$NAMED and N_REPLICAS=$N_REPLICAS in REALM=$REALM"

# Make the ux root dir.
mkdir -p $UXROOT

$PRIVILEGED_BIN/realm/boot $REALM

./mount.sh
