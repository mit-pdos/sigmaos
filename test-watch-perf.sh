rm -f /tmp/sigmaos-perf/*
# export SIGMADEBUG=""
# export SIGMADEBUG="WATCH_DEBUG_PERF"
# export SIGMADEBUG="WATCH;WATCH_V2;WATCH_PERF;WATCH_TEST"
# export SIGMAPERF="WATCH_TEST_WORKER_PPROF;WATCH_TEST_WORKER_PPROF_MUTEX;WATCH_TEST_WORKER_PPROF_BLOCK;WATCH_PERF_WORKER_PPROF;UX_PPROF;WATCH_PERF_WORKER_PPROF_MUTEX;UX_PPROF_MUTEX"

export S3_BUCKET="sigmaos-bucket-ryan/$(date +%Y-%m-%d_%H:%M:%S)"

DIRREADER_VERSIONS=("1" "2")
MEASURE_MODES=("watch_only" "include_op")
USE_NAMEDS=("0" "1")
NUM_WORKERS=("1" "10" "25")
NUM_STARTING_FILES=("0" "100" "1000")

for DIRREADER_VERSION in "${DIRREADER_VERSIONS[@]}"; do
  for MEASURE_MODE in "${MEASURE_MODES[@]}"; do
    for USE_NAMED in "${USE_NAMEDS[@]}"; do
      for STARTING_FILES in "${NUM_STARTING_FILES[@]}"; do
        ./stop.sh
        ./stop-etcd.sh
        ./start-etcd.sh

        DIRREADER_VERSION="$DIRREADER_VERSION" \
        MEASURE_MODE="$MEASURE_MODE" \
        USE_NAMED="$USE_NAMED" \
        NUM_WORKERS="1" \
        NUM_STARTING_FILES="$STARTING_FILES" \
        go test sigmaos/fslib/dirreader -v --start --run "TestPerf" --timeout 5m

        if [ $? -ne 0 ]; then
          echo "Error encountered. Exiting."
          exit 1
        fi
      done
    done
  done
done

for DIRREADER_VERSION in "${DIRREADER_VERSIONS[@]}"; do
  for MEASURE_MODE in "${MEASURE_MODES[@]}"; do
    for USE_NAMED in "${USE_NAMEDS[@]}"; do
      for WORKERS in "${NUM_WORKERS[@]}"; do
        ./stop.sh
        ./stop-etcd.sh
        ./start-etcd.sh

        DIRREADER_VERSION="$DIRREADER_VERSION" \
        MEASURE_MODE="$MEASURE_MODE" \
        USE_NAMED="$USE_NAMED" \
        NUM_WORKERS="$WORKERS" \
        NUM_STARTING_FILES="0" \
        go test sigmaos/fslib/dirreader -v --start --run "TestPerf" --timeout 5m

        if [ $? -ne 0 ]; then
          echo "Error encountered. Exiting."
          exit 1
        fi
      done
    done
  done
done
