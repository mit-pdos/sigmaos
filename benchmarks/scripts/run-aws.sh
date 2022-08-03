#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --realm1 REALM1 --realm2 REALM2 [--version VERSION]" 1>&2
}

VPC=""
REALM1=""
REALM2=""
VERSION=$(date +%s)
while [[ "$#" -gt 0 ]]; do
  case "$1" in
  --vpc)
    shift
    VPC="$1"
    shift
    ;;
  --realm1)
    shift
    REALM1=$1
    shift
    ;;
  --realm2)
    shift
    REALM2=$1
    shift
    ;;
  --version)
    shift
    VERSION=$1
    shift
    ;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "unexpected argument $1"
    usage
    exit 1
    ;;
  esac
done

if [ -z "$VPC" ] || [ -z "$REALM1" ] || [ -z "$REALM2" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

# Set some variables
DIR=$(realpath $(dirname $0)/../..)
. $DIR/.env
AWS_DIR=$DIR/aws
OUT_DIR=$DIR/benchmarks/results/$VERSION

# cd to the ulambda root directory
cd $DIR
mkdir $OUT_DIR

# ========== Helpers ==========

start_cluster() {
  if [ $# -ne 1 ]; then
    echo "start_cluster requries one argument: n_vm" 1>&2
    exit 1
  fi
  n_vm=$1
  cd $AWS_DIR
  ./stop-sigmaos.sh --vpc $VPC --parallel
  ./build-sigma.sh --vpc $VPC --realm $REALM1 --version $VERSION
  ./build-sigma.sh --vpc $VPC --realm $REALM2 --version $VERSION
  ./install-sigma.sh --vpc $VPC --realm $REALM1 --parallel
  ./install-sigma.sh --vpc $VPC --realm $REALM2 --parallel
  ./start-sigmaos.sh --vpc $VPC --realm $REALM1 --n $n_vm
  cd $DIR
}

run_benchmark() {
  if [ $# -ne 2 ]; then
    echo "run_benchmark args: perfdir cmd" 1>&2
    exit 1
  fi
  perf_dir=$1
  cmd=$2
  mkdir -p $perf_dir
  cd $AWS_DIR
  ./run-benchmark.sh --vpc $VPC --command "$cmd"
  ./collect-results.sh --vpc $VPC --perfdir $perf_dir --parallel
  cd $DIR
}

run_mr() {
  if [ $# -ne 2 ]; then
    echo "run_mr args: app perf_dir" 1>&2
    exit 1
  fi
  mrapp=$1
  perf_dir=$2
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppMR --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $perf_dir "$cmd"
}

run_kv() {
  if [ $# -ne 3 ]; then
    echo "run_kv args: nkvd nclerk perf_dir" 1>&2
    exit 1
  fi
  nkvd=$1
  nclerk=$2
  perf_dir=$3
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppKVUnrepl --nkvd $nkvd --nclerk $nclerk > /tmp/bench.out 2>&1
  "
  run_benchmark $perf_dir "$cmd"
}

# ========== Top-level benchmarks ==========

mr_scalability() {
  mrapp=mr-grep-wiki.yml
  for n_vm in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/$n_vm
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    start_cluster $n_vm
    run_mr $mrapp $perf_dir
  done
}

mr_vs_corral() {
  mrapp=mr-wc-wiki1.8G.yml
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  start_cluster 16
  run_mr $mrapp $perf_dir
}

mr_overlap() {
  mrapp=mr-wc-wiki4G.yml
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  start_cluster 16
  run_mr $mrapp $perf_dir
}

kv_scalability() {
  nkvd=1
  for nclerk in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/$nclerk
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    start_cluster 16
    run_kv $nkvd $nclerk $perf_dir
  done
}

kv_elasticity() {
  # TODO
  echo "TODO"
}

realm_burst() {
  run=${FUNCNAME[0]}
  perf_dir=$OUT_DIR/$run
  start_cluster 16
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run RealmBurst > /tmp/bench.out 2>&1
  "
  run_benchmark $perf_dir "$cmd"
}

realm_balance() {
  run=${FUNCNAME[0]}
  perf_dir=$OUT_DIR/$run
  start_cluster 16
  cmd="
    $PRIVILEGED_BIN/realm/create $REALM2; \
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run Balance > /tmp/bench.out 2>&1
  "
  run_benchmark $perf_dir "$cmd"
}

# ========== Run benchmarks ==========
mr_scalability
mr_vs_corral
mr_overlap
kv_scalability
realm_burst
realm_balance

# ========== Produce graphs ==========
