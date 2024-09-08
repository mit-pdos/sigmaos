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

if [ $EXP != "all" ] && [ $EXP != "fig_6" ] && [ $EXP != "fig_10" ] && [ $EXP != "fig_12" ]; then
  echo "Unkown experiment $EXP"
  usage
  exit 1
fi

VERSION=SOSP24AE

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

mkdir -p $LOG_DIR

# Figure 6
if [ $EXP == "all" ] || [ $EXP == "fig_6" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 6 data..."
    rm -rf benchmarks/results/$VERSION/cold_start
  fi
  echo "Generating Figure 6 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestColdStart --parallelize --platform aws --vpc $AWS_VPC_SMALL --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig6.out
  echo "Done generating Figure 6 data..."
fi

# Figure 10
if [ $EXP == "all" ] || [ $EXP == "fig_10" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 10 data..."
    rm -rf benchmarks/results/$VERSION/mr_vs_corral
  fi
  echo "Generating Figure 10 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestMR --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig10-mr.out
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestCorral --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig10-corral.out
  echo "Done generating Figure 10 data..."
fi

# Figure 12
if [ $EXP == "all" ] || [ $EXP == "fig_12" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 12 data..."
    rm -rf benchmarks/results/$VERSION/be_imgresize_multiplexing
  fi
  echo "Generating Figure 12 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestBEImgresizeMultiplexing --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig12.out
  echo "Done generating Figure 12 data..."
fi
