#!/bin/bash

usage() {
  echo "Usage: $0 --realm REALM [--profile PROFILE] [--version VERSION]" 1>&2
}

REALM=""
PROFILE=""
VERSION=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
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
  --version)
    shift
    VERSION=$1
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

if [ -z "$REALM" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
. $DIR/.env

if [ -z "$VERSION" ]; then
  VERSION=$(cat $VERSION_FILE)
fi

# Copy kernel & realm builds to S3
aws s3 cp --recursive bin/realm s3://$REALM/bin/realm $PROFILE
aws s3 cp --recursive bin/kernel s3://$REALM/bin/kernel $PROFILE

# Copy versioned user procs to s3.
aws s3 cp --recursive bin/user s3://$REALM/bin/user/$VERSION $PROFILE
