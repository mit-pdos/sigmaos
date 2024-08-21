#!/bin/bash

VERSION=SOSP24AE

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=XXX
AWS_VPC_LARGE=YYY

# TODO: get VPCs right

mkdir -p $LOG_DIR

# Figure 6
echo "Generating Figure 6 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestColdStart --parallelize --platform aws --vpc $AWS_VPC_SMALL --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig6.out
echo "Done generating Figure 6 data..."

# Figure 8
echo "Generating Figure 8 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSchedScalability --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig8.out
echo "Done generating Figure 8 data..."

# Figure 10
echo "Generating Figure 10 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestMR --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig10-mr.out
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestCorral --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig10-corral.out
echo "Done generating Figure 10 data..."

# Figure 11
echo "Generating Figure 11 data..."
echo "FIG 11 TODO"
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestHotelTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig11-hotel.out
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestSocialnetTailLatency --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig11-socialnet.out
echo "Done generating Figure 11 data..."

# Figure 12
echo "Generating Figure 12 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestBEImgresizeMultiplexing --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig12.out
echo "Done generating Figure 12 data..."

# Figure 13
echo "Generating Figure 13 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestLCBEHotelImgresizeMultiplexing --parallelize --platform cloudlab --vpc none --tag sosp24ae --no-shutdown --version $VERSION 2>&1 | tee $LOG_DIR/fig13.out
echo "Done generating Figure 13 data..."
