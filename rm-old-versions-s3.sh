#!/bin/bash

usage() {
  echo "Usage: $0 --tag TAG [--profile PROFILE] [--parallel]" 1>&2
}

TAG=""
PROFILE=""
PARALLEL=""
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
  --parallel)
    shift
    PARALLEL="--parallel"
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

# Get names of old bins
oldbins=$(aws s3 ls --recursive s3://$TAG/bin $PROFILE | awk '{print $NF}')

if [ "$oldbins" == "" ]; then
  echo "Nothing to remove"
  exit 0
fi

if [ -z "$PARALLEL" ]; then
  # If not removing in parallel, remove with 1 thread
  njobs=1
else
  # If removing up in parallel, remove with (n - 1) threads.
  njobs=$(nproc)
  njobs="$(($njobs-1))"
fi

rmbins="parallel -j$njobs aws \"s3 rm s3://$TAG/{}\" ::: $oldbins"
echo $rmbins
eval $rmbins
