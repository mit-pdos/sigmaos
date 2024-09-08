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

if [ $EXP != "all" ] && [ $EXP != "fig_8" ] && [ $EXP != "fig_11" ] && [ $EXP != "fig_13" ]; then
  echo "Unkown experiment $EXP"
  usage
  exit 1
fi

VERSION=SOSP24AE

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

mkdir -p $LOG_DIR

# Figure 8
if [ $EXP == "all" ] || [ $EXP == "fig_8" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 8 data..."
    rm -rf benchmarks/results/$VERSION/sched_scalability
  fi
  echo "Generating Figure 8 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSchedScalability --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig8.out
  echo "Done generating Figure 8 data..."
fi

# Figure 11
if [ $EXP == "all" ] || [ $EXP == "fig_11" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 11 data..."
    rm -rf benchmarks/results/$VERSION/hotel_tail_latency
    rm -rf benchmarks/results/$VERSION/socialnet_tail_latency
  fi
  echo "Generating Figure 11 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestHotelTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig11-hotel.out
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSocialnetTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig11-socialnet.out
  echo "Done generating Figure 11 data..."
fi

# Figure 13
if [ $EXP == "all" ] || [ $EXP == "fig_13" ]; then
  if [ $RERUN == "true" ]; then
    echo "Clearing any cached Figure 13 data..."
    rm -rf benchmarks/results/$VERSION/lc_be_hotel_imgresize_multiplexing
  fi
  echo "Generating Figure 13 data..."
  go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestLCBEHotelImgresizeMultiplexing --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig13.out
  echo "Done generating Figure 13 data..."
fi
