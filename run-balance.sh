#!/bin/bash

cd aws

VPC=vpc-02f7e3816c4cc8e7f

./stop-sigmaos.sh --vpc $VPC
./build-sigma.sh --vpc vpc-02f7e3816c4cc8e7f --realm arielck && ./build-sigma.sh --vpc vpc-02f7e3816c4cc8e7f --realm test-realm && ./install-sigma.sh --vpc vpc-02f7e3816c4cc8e7f --realm arielck
./start-sigmaos.sh --vpc $VPC --realm arielck

vms=(`./lsvpc.py $VPC | grep -w VMInstance | cut -d " " -f 5`)
MAIN="${vms[0]}"

scp -i key-$VPC.pem ubuntu@$MAIN:~/ulambda/VERSION.txt ../VERSION.txt

echo "Run: $MAIN"
ssh -i key-$VPC.pem ubuntu@$MAIN /bin/bash <<ENDSSH
cd ulambda
export NAMED=10.0.44.163:1111
export SIGMAPERF="KVCLERK_TPT;MR-MAPPER_TPT;MR-REDUCER_TPT;"
./bin/realm/create test-realm
go clean -testcache; go test -v ulambda/benchmarks --version=$(cat ../VERSION.txt) -run Balance --realm arielck -timeout 0 > /tmp/bench 2>&1
ENDSSH
./collect-results.sh --vpc $VPC
