#!/bin/bash

LOG_DIR="/tmp/sigmaos-test-logs"
TEST_OUT_DIR="/tmp/sigmaos-nightly-test-output"
rm -rf $TEST_OUT_DIR
mkdir $TEST_OUT_DIR

SIGMAOS_ROOT=$HOME/sigmaos
for branch in $(cat $SIGMAOS_ROOT/branches-to-test.txt); do
  BRANCH_OUT_DIR="$TEST_OUT_DIR/$branch"
  mkdir $BRANCH_OUT_DIR
  OUT_FILE="$BRANCH_OUT_DIR/run.out"
  touch $OUT_FILE
  echo "=== Checking out to branch $branch" | tee -a $OUT_FILE
  git checkout $branch
  ./build.sh 2>&1 | tee -a $OUT_FILE
  echo "=== Building branch $branch" | tee -a $OUT_FILE
  ./stop.sh --parallel > /dev/null
  echo "=== Running tests on branch $branch" | tee -a $OUT_FILE
  # Run tests
  ./test.sh --cleanup --savelogs 2>&1 | tee -a $OUT_FILE
  # Copy log output
  mv $LOG_DIR/ $BRANCH_OUT_DIR/
  echo "=== Done running tests on branch $branch" | tee -a $OUT_FILE
  ./stop.sh --parallel > /dev/null
done
