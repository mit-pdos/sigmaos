#!/bin/bash

usage() {
  echo "Usage: $0 [--exp fig_XXX] [--rerun]" 1>&2
}

EXP="all"
RERUN="false"
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --exp)
    shift
    EXP=$1
    shift
    ;;
  --rerun)
    shift
    RERUN="true"
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

if [ $# -gt 0 ]; then
    usage
    exit 1
fi

if [ $EXP != "all" ] && [ $EXP != "cgroups" ]; then
  echo "Unkown experiment $EXP"
  usage
  exit 1
fi

VERSION="CGROUPS"
BRANCH="test-in-docker"
TAG="arielck"

LOG_DIR=/tmp/sigmaos-experiment-logs

mkdir -p $LOG_DIR

# Figure 13
if [ $EXP == "all" ] || [ $EXP == "cgroups" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached cgroups experiment data..."
    rm -rf benchmarks/results/$VERSION/lc_be_hotel_imgresize_rpc_multiplexing
  fi
  echo "Generating cgroups experiment data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestLCBEHotelImgResizeRPCMultiplexing --parallelize --platform cloudlab --vpc none --tag $TAG --no-shutdown --version $VERSION --branch $BRANCH 2>&1 | tee $LOG_DIR/cgroups.out
  echo "Done generating cgroups experiment data..."
fi
