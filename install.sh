#!/bin/bash

# Install the sigmaOS software, either from the local build or from s3.

usage() {
    echo "Usage: $0 [--from FROM] [--realm REALM] [--profile PROFILE]" 1>&2
}

FROM="local"
REALM="test-realm"
PROFILE=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --from)
    shift
    FROM="$1"
    shift
    ;;
  --realm)
    shift
    REALM=$1
    shift
    ;;
  --profile)
    shift
    PROFILE="--profile $1"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/.env

mkdir -p $BIN
rm -rf $BIN/*
if [ $FROM == "local" ]; then
  # Make the user program dir
  mkdir -p $UXROOT/$REALM/bin
  # Copy from local
  cp -r bin/user $UXROOT/$REALM/bin
  cp -r bin/realm $BIN
  cp -r bin/kernel $BIN
elif [ $FROM == "s3" ]; then
  # Copy kernel & realm dirs from s3
  aws s3 cp --recursive s3://$REALM/bin/realm $BIN/realm $PROFILE
  aws s3 cp --recursive s3://$REALM/bin/kernel $BIN/kernel $PROFILE
  chmod --recursive +x $BIN
  mkdir $BIN/user
else
  echo "Unrecognized bin source: $FROM"
  exit 1
fi
