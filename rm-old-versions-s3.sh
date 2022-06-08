#!/bin/bash

usage() {
  echo "Usage: $0 [--realm REALM] [--profile PROFILE]" 1>&2
}

REALM="test-realm"
PROFILE=""
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

VERSION=$(cat "${VERSION_FILE}")
oldbins=$(aws s3 ls --recursive s3://$REALM/bin/user $PROFILE | awk '{print $NF}')
echo $oldbins

for bin in $oldbins; do
  if ! [[ $bin == *$VERSION* ]]; then
    echo rm $bin
    aws s3 rm s3://$REALM/$bin &
  fi
done
wait
