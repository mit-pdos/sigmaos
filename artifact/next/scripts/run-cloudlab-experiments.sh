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

if [ $EXP != "all" ] && [ $EXP != "cossim" ]; then
  echo "Unkown experiment $EXP"
  usage
  exit 1
fi

VERSION=NEXT
TAG=arielck

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

mkdir -p $LOG_DIR

# Figure 8
if [ $EXP == "all" ] || [ $EXP == "cossim" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached CosSim data..."
    rm -rf benchmarks/results/$VERSION/cos_sim_tail_latency_*
  fi
  echo "Generating CosSim data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestScaleCosSim --parallelize --platform cloudlab --vpc none --tag $TAG --no-shutdown --version $VERSION --branch cpp 2>&1 | tee $LOG_DIR/cossim.out
  echo "Done generating CosSim data..."
fi
