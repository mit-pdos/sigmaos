#!/bin/bash

VERSION=SOSP24AE

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

mkdir -p $LOG_DIR

# Figure 8
echo "Generating Figure 8 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSchedScalability --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig8.out
echo "Done generating Figure 8 data..."

# Figure 11
echo "Generating Figure 11 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestHotelTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig11-hotel.out
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSocialnetTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig11-socialnet.out
echo "Done generating Figure 11 data..."

# Figure 13
echo "Generating Figure 13 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestLCBEHotelImgresizeMultiplexing --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig13.out
echo "Done generating Figure 13 data..."
