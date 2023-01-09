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
. $DIR/env/env.sh

if [[ -z "${SIGMANAMED}" ]]; then
  export SIGMANAMED=":1111"
fi

if [[ -z "${N_REPLICAS}" ]]; then
  export N_REPLICAS=1
fi

echo "running with SIGMANAMED=$SIGMANAMED and N_REPLICAS=$N_REPLICAS in REALM=$REALM"

bootsys $REALM &

sleep 1

# ./mount.sh
