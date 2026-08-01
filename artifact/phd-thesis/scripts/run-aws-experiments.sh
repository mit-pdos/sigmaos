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

if [ $EXP != "all" ] && [ $EXP != "mr" ] && [ $EXP != "mr_multi" ] && [ $EXP != "corral" ]; then
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

if [ $EXP == "all" ] || [ $EXP == "corral" ]; then
  echo "Generating Corral data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestCorral --parallelize --platform aws --vpc $AWS_VPC --build-tag $TAG --no-shutdown-after-test --bench-version $VERSION --branch $BRANCH 2>&1 | tee $LOG_DIR/mr.out
  echo "Done generating Corral data..."
fi

if [ $EXP == "all" ] || [ $EXP == "mr" ]; then
  echo "Generating MR data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestMR --parallelize --platform aws --vpc $AWS_VPC --build-tag $TAG --no-shutdown-after-test --bench-version $VERSION --branch $BRANCH 2>&1 | tee $LOG_DIR/mr.out
  echo "Done generating MR data..."
fi

if [ $EXP == "all" ] || [ $EXP == "mr_multi" ]; then
  # Mem request per mapper is what bounds how many mappers run concurrently
  # per node (msched admits on memory, and a mapper's real RSS is far below
  # any of these values), so sweeping it sweeps the packing:
  #   8000 -> ~1/node, 3000 -> ~5/node, 1500 -> ~10/node, 1200 -> ~13/node.
  # Override with e.g. MR_MULTI_MEM_REQS="3000 1200" MR_MULTI_GOMAXPROCS="0 2".
  MR_MULTI_MEM_REQS=${MR_MULTI_MEM_REQS:-"1200"}
  MR_MULTI_GOMAXPROCS=${MR_MULTI_GOMAXPROCS:-"0"}
  for gmp in $MR_MULTI_GOMAXPROCS; do
    for mem in $MR_MULTI_MEM_REQS; do
      echo "Generating MR data (mem_req=$mem gomaxprocs=$gmp)..."
      go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestBEMRMultiplexing --parallelize --platform aws --vpc $AWS_VPC --build-tag $TAG --no-shutdown-after-test --bench-version $VERSION --branch $BRANCH --mr_mem_req $mem --mr_gomaxprocs $gmp 2>&1 | tee $LOG_DIR/mr_multi_mem${mem}_gomaxprocs${gmp}.out
      echo "Done generating MR data (mem_req=$mem gomaxprocs=$gmp)..."
    done
  done
fi
