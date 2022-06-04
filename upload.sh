#!/bin/bash

usage() {
  echo "Usage: $0 [--profile PROFILE]" 1>&2
}

PROFILE=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi



DIR=$(dirname $0)
. $DIR/.env

# Copy to S3
aws s3 cp --recursive bin s3://9ps3/bin $PROFILE
