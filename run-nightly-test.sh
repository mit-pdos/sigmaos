#!/bin/bash

LOG_DIR="/tmp/sigmaos-test-logs"
TEST_OUT_DIR="/tmp/sigmaos-nightly-test-output"
rm -rf $TEST_OUT_DIR
mkdir $TEST_OUT_DIR

SIGMAOS_ROOT=$HOME/sigmaos
for branch in $(cat $SIGMAOS_ROOT/branches-to-test.txt); do
  BRANCH_OUT_DIR="$TEST_OUT_DIR/$branch"
  mkdir $BRANCH_OUT_DIR
  ./stop.sh --parallel > /dev/null
  echo "=== Running tests on branch $branch"
  # Run tests
  ./test.sh --cleanup --savelogs
  # Copy log output
  cp -r $LOG_DIR/* $BRANCH_OUT_DIR/
  echo "=== Done running tests on branch $branch"
  ./stop.sh --parallel > /dev/null
done
