#!/bin/bash

usage() {
  echo "Usage: $0 --manifest PATH --username USERNAME" 1>&2
}

MANIFEST=""
USERNAME=""
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --manifest)
    shift
    MANIFEST=$1
    shift
    ;;
  --username)
    shift
    USERNAME=$1
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

if [ -z "$USERNAME" ] || [ -z "$MANIFEST" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

DIR=$(dirname $0)
source $DIR/env.sh

servers=$(grep "username=\"$USERNAME" $MANIFEST | cut -d "\"" -f4)

rm $DIR/servers.txt

i=0
for s in $servers ; do
  echo "node$i $s" | tee -a $DIR/servers.txt
  i=$(($i + 1))
done

