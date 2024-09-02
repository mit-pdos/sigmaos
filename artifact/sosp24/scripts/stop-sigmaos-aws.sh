#!/bin/bash

ROOT_DIR=$(realpath $(dirname $0)/../../..)
SCRIPTS_DIR=$ROOT_DIR/aws

AWS_VPC_SMALL=vpc-0814ec9c0d661bffc
AWS_VPC_LARGE=vpc-0affa7f07bd923811

cd $SCRIPTS_DIR
./stop-sigmaos.sh --vpc $AWS_VPC_SMALL --parallel
./stop-sigmaos.sh --vpc $AWS_VPC_LARGE --parallel
