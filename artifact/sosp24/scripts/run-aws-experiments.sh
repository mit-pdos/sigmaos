#!/bin/bash

VERSION=SOSP24AE

LOG_DIR=/tmp/sigmaos-experiment-logs

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

mkdir -p $LOG_DIR

# Figure 6
echo "Generating Figure 6 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestColdStart --parallelize --platform aws --vpc $AWS_VPC_SMALL --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig6.out
echo "Done generating Figure 6 data..."

# Figure 10
echo "Generating Figure 10 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestMR --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig10-mr.out
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestCorral --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig10-corral.out
echo "Done generating Figure 10 data..."

# Figure 12
echo "Generating Figure 12 data..."
go clean -testcache; go test -v -timeout 0 sigmaos/benchmarks/remote --run TestBEImgresizeMultiplexing --parallelize --platform aws --vpc $AWS_VPC_LARGE --tag sosp24ae --no-shutdown --version $VERSION --branch sosp24ae 2>&1 | tee $LOG_DIR/fig12.out
echo "Done generating Figure 12 data..."
