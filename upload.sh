#!/bin/bash

usage() {
  echo "Usage: $0 --tag TAG [--profile PROFILE]" 1>&2
}

TAG=""
PROFILE=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --tag)
    shift
    TAG=$1
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
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$TAG" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/env/env.sh

# Copy versioned user procs to s3.
aws s3 cp --recursive bin/user s3://$TAG/bin $PROFILE

# Copy WASM scripts to S3
aws s3 cp --recursive bin/wasm s3://$TAG/wasm $PROFILE
