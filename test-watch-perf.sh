rm -f /tmp/sigmaos-perf/*
./stop.sh
./stop-etcd.sh
./start-etcd.sh
export SIGMADEBUG=""
# export SIGMADEBUG="WATCH;WATCH_V2;WATCH_PERF;WATCH_TEST"
# export SIGMAPERF="WATCH_TEST_WORKER_PPROF;WATCH_TEST_WORKER_PPROF_MUTEX;WATCH_TEST_WORKER_PPROF_BLOCK;WATCH_PERF_WORKER_PPROF"
export DIRREADER_VERSION="1"
export WATCHPERF_MEASURE_MODE="1"
export S3_BUCKET="sigmaos-bucket-ryan"
go test sigmaos/fslib/dirreader -v --start
