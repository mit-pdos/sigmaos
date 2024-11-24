rm -f /tmp/sigmaos-perf/*
./stop.sh
./stop-etcd.sh
./start-etcd.sh
export SIGMADEBUG=""
# export SIGMADEBUG="WATCH;WATCH_V2;WATCH_PERF;WATCH_TEST"
# export SIGMAPERF="WATCH_TEST_WORKER_PPROF;WATCH_TEST_WORKER_PPROF_MUTEX;WATCH_TEST_WORKER_PPROF_BLOCK;WATCH_PERF_WORKER_PPROF;UX_PPROF;WATCH_PERF_WORKER_PPROF_MUTEX;UX_PPROF_MUTEX"
export DIRREADER_VERSION="1"
export S3_BUCKET="sigmaos-bucket-ryan/$(date +%Y-%m-%d_%H:%M:%S)"
go test sigmaos/fslib/dirreader -v --start --run "Perf" --timeout 0
