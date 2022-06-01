#!/bin/bash

# Install the sigmaOS software, either from the local build or from s3.

usage() {
    echo "Usage: $0 [-from FROM]" 1>&2
}

. ./.env

FROM="local"
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  -from)
    shift
    FROM="$1"
    shift
    ;;
  -h)
    usage
    exit 0
    ;;
  --help)
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

mkdir -p $BIN
rm -rf $BIN/*
if [ $FROM == "local" ]; then
  # Copy from local
  cp -r bin/* $BIN
elif [ $FROM == "s3" ]; then
  # Copy from s3
  echo "cp from s3"
  aws s3 cp --recursive s3://9ps3/bin $BIN
  chmod --recursive +x $BIN
else
  echo "Unrecognized bin source: $FROM"
  exit 1
fi
