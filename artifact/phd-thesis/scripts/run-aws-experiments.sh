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

if [ $EXP != "all" ] && [ $EXP != "mr" ] && [ $EXP != "mr_multi" ]; then
  echo "Unkown experiment $EXP"
  usage
  exit 1
fi

VERSION=PHD_THESIS
TAG=arielck
BRANCH=master

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC=vpc-02f7e3816c4cc8e7f

mkdir -p $LOG_DIR

if [ $EXP == "all" ] || [ $EXP == "mr" ]; then
  echo "Generating MR data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestMR --parallelize --platform aws --vpc $AWS_VPC --build-tag $TAG --no-shutdown-after-test --bench-version $VERSION --branch $BRANCH 2>&1 | tee $LOG_DIR/mr.out
  echo "Done generating MR data..."
fi

if [ $EXP == "all" ] || [ $EXP == "mr_multi" ]; then
  echo "Generating MR data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestBEMRMultiplexing --parallelize --platform aws --vpc $AWS_VPC --build-tag $TAG --no-shutdown-after-test --bench-version $VERSION --branch $BRANCH 2>&1 | tee $LOG_DIR/mr.out
  echo "Done generating MR data..."
fi
