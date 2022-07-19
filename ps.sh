#!/bin/bash

usage() {
  echo "Usage: $0 --realm REALM" 1>&2
}

REALM=""
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --realm)
    shift
    REALM="$1"
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

for f in /mnt/9p/realm-nameds/$REALM/procd/*:*
do
  echo "===" $f
  find "$f/running/" -type f -print | xargs -I {} jq -rc '.Program,.Args' {}
done
