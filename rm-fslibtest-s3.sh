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
  
files=$(aws s3 ls --recursive s3://9ps3/fslibtest $PROFILE | awk '{print $NF}')

for f in $files; do
    aws s3 rm s3://9ps3/$f 
done
wait
