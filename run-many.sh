#!/bin/bash

usage() {
  echo "Usage: $0 PKG [TEST]" 1>&2
  exit 1
}

if [ $# -lt 1 ]; then
    usage
fi

PKG=$1
N=20
TEST=""
if [ $# -gt 1 ]; then
    TEST="--run $2"
fi

for i in $(seq 1 $N);	 
do
    echo "=== run $i"
    ./stop.sh && ./test-in-docker.sh  --pkg $PKG $TEST 2>&1 | tee test.out
    grep FAIL test.out
    if [ $? -eq 0 ]; then
	echo "TEST FAILED"
	exit 1
    fi
done
