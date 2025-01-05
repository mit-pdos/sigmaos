#!/bin/bash

export SIGMADEBUG="WATCH_PERF"
# export SIGMADEBUG="WATCH;WATCH_V2;WATCH_PERF;WATCH_TEST"
# export SIGMAPERF="WATCH_TEST_WORKER_PPROF;WATCH_TEST_WORKER_PPROF_MUTEX;WATCH_TEST_WORKER_PPROF_BLOCK;WATCH_PERF_WORKER_PPROF;UX_PPROF;WATCH_PERF_WORKER_PPROF_MUTEX;UX_PPROF_MUTEX"

export S3_BUCKET="sigmaos-bucket-ryan/$(date +%Y-%m-%d_%H:%M:%S)"

DIRREADER_VERSIONS=("1" "2")
MEASURE_MODES=("watch_only" "include_op")
USE_NAMEDS=("0" "1")
NUM_STARTING_FILES=("0" "100" "500" "1000")

NUM_WORKERS=("1" "5" "10" "15")
NUM_FILES_PER_TRIAL=("1" "5" "10" "15")

retry_until_success() {
  local log_prefix=$1
  local max_retries=5
  local retry_count=0

  while [ $retry_count -lt $max_retries ]; do
    ./stop.sh
    ./fsetcd-wipe.sh
    go test sigmaos/sigmaclnt/fslib/dirreader -v --start --run "TestPerf" --timeout 15m
    if [ $? -eq 0 ]; then
      echo "Test succeeded after $retry_count retries."
      return 0
    else
      echo "Error encountered. Saving logs and retrying..."
      local log_file="${log_prefix}_retry_${retry_count}_$(date +%Y-%m-%d_%H:%M:%S).log"
      ./logs.sh > "$log_file"
      echo "Logs saved to $log_file"
      retry_count=$((retry_count + 1))
    fi
  done

  echo "Test failed after $max_retries retries. Moving on."
  return 1
}

for DIRREADER_VERSION in "${DIRREADER_VERSIONS[@]}"; do
  for STARTING_FILES in "${NUM_STARTING_FILES[@]}"; do
    for WORKERS in "${NUM_WORKERS[@]}"; do
      for FILES_PER_TRIAL in "${NUM_FILES_PER_TRIAL[@]}"; do
        for MEASURE_MODE in "${MEASURE_MODES[@]}"; do
          for USE_NAMED in "${USE_NAMEDS[@]}"; do
            if (( STARTING_FILES > 0 )) && (( WORKERS > 1 || FILES_PER_TRIAL > 1 )); then
              continue
            fi
            
            NUM_TRIALS=$((150 / FILES_PER_TRIAL))
            
            DIRREADER_VERSION="$DIRREADER_VERSION" \
            MEASURE_MODE="$MEASURE_MODE" \
            USE_NAMED="$USE_NAMED" \
            NUM_WORKERS="$WORKERS" \
            NUM_TRIALS="$NUM_TRIALS" \
            NUM_FILES_PER_TRIAL="$FILES_PER_TRIAL" \
            NUM_STARTING_FILES="$STARTING_FILES" \
            retry_until_success "dirreader_${DIRREADER_VERSION}_${MEASURE_MODE}_${USE_NAMED}_${STARTING_FILES}_${WORKERS}_${FILES_PER_TRIAL}"
          done
        done
      done
    done
  done
done
