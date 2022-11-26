#!/bin/bash

# Install the sigmaOS software, either from the local build or from s3.

usage() {
    echo "Usage: $0 --realm REALM [--from FROM] [--profile PROFILE]" 1>&2
}

FROM="local"
REALM=""
VERSION=""
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
  --version)
    shift
    VERSION=$1
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

if [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/.env

mkdir -p $PRIVILEGED_BIN
rm -rf $PRIVILEGED_BIN/*
rm -rf $UXROOT/$REALM/bin/user/*
if [ $FROM == "local" ]; then
  if [ -z "$VERSION" ]; then
    VERSION=$(cat "${VERSION_FILE}")
  fi
  # Make the user program dir
  mkdir -p $UXROOT/$REALM/bin/user/$VERSION/
  # Copy from local
  cp -r bin/user/* $UXROOT/$REALM/bin/user/$VERSION/
  cp -r bin/realm $PRIVILEGED_BIN
  cp -r bin/kernel $PRIVILEGED_BIN
elif [ $FROM == "s3" ]; then
  # Copy kernel & realm dirs from s3
  aws s3 cp --recursive s3://$REALM/bin/realm $PRIVILEGED_BIN/realm $PROFILE
  aws s3 cp --recursive s3://$REALM/bin/kernel $PRIVILEGED_BIN/kernel $PROFILE
  chmod --recursive +x $PRIVILEGED_BIN
else
  echo "Unrecognized bin source: $FROM"
  exit 1
fi
