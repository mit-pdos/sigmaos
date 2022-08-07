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

if [ -z "$VPC" ] || [ -z "$REALM1" ] || [ -z "$REALM2" ] || [ -z "$VERSION" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

# Set some variables
DIR=$(realpath $(dirname $0)/../..)
. $DIR/.env
AWS_DIR=$DIR/aws
OUT_DIR=$DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$DIR/benchmarks/results/graphs
INIT_OUT=/tmp/init.out

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
  echo "" > $INIT_OUT
  ./stop-sigmaos.sh --vpc $VPC --parallel >> $INIT_OUT 2>&1
  ./build-sigma.sh --vpc $VPC --realm $REALM1 --version $VERSION >> $INIT_OUT 2>&1
  ./build-sigma.sh --vpc $VPC --realm $REALM2 --version $VERSION >> $INIT_OUT 2>&1
  ./install-sigma.sh --vpc $VPC --realm $REALM1 --parallel >> $INIT_OUT 2>&1
  ./install-sigma.sh --vpc $VPC --realm $REALM2 --parallel >> $INIT_OUT 2>&1
  ./start-sigmaos.sh --vpc $VPC --realm $REALM1 --n $n_vm >> $INIT_OUT 2>&1
  cd $DIR
}

run_benchmark() {
  if [ $# -ne 3 ]; then
    echo "run_benchmark args: n_vm perfdir cmd" 1>&2
    exit 1
  fi
  n_vm=$1
  perf_dir=$2
  cmd=$3
  # Avoid doing duplicate work.
  if [ -d $perf_dir ]; then
    echo "========== Already ran, skipping: $perf_dir =========="
    return 0
  fi
  start_cluster $n_vm
  mkdir -p $perf_dir
  cd $AWS_DIR
  ./run-benchmark.sh --vpc $VPC --command "$cmd"
  ./collect-results.sh --vpc $VPC --perfdir $perf_dir --parallel >> $INIT_OUT 2>&1
  cd $DIR
}

run_mr() {
  if [ $# -ne 3 ]; then
    echo "run_mr args: n_vm app perf_dir" 1>&2
    exit 1
  fi
  n_vm=$1
  mrapp=$2
  perf_dir=$3
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppMR --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $n_vm $perf_dir "$cmd"
}

run_kv() {
  if [ $# -ne 6 ]; then
    echo "run_kv args: n_vm nkvd nclerk auto redisaddr perf_dir" 1>&2
    exit 1
  fi
  n_vm=$1
  nkvd=$2
  nclerk=$3
  auto=$4
  redisaddr=$5
  perf_dir=$6
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppKVUnrepl --nkvd $nkvd --nclerk $nclerk --kvauto $auto --redisaddr \"$redisaddr\" > /tmp/bench.out 2>&1
  "
  run_benchmark $n_vm $perf_dir "$cmd"
}

# ========== Top-level benchmarks ==========

mr_scalability() {
  mrapp=mr-grep-wiki.yml
  for n_vm in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/sigmaOS/$n_vm
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr $n_vm $mrapp $perf_dir
  done
}

mr_vs_corral() {
  n_vm=16
  app="mr-wc-wiki"
  dataset_size="1G 1.8G 2G 4G"
  for size in $dataset_size ; do
    mrapp="$app$size.yml"
    run=${FUNCNAME[0]}/$mrapp
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr $n_vm $mrapp $perf_dir
  done
}

mr_overlap() {
  mrapp=mr-wc-wiki4G.yml
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  # TODO
  echo "TODO"
#  run_mr $n_vm $mrapp $perf_dir
}

kv_scalability() {
  # First, run against our KV.
  auto="manual"
  nkvd=1
  redisaddr=""
  n_vm=16
  for nclerk in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/sigmaOS/$nclerk
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_kv $n_vm $nkvd $nclerk $auto "$redisaddr" $perf_dir
  done

  # Then, run against a redis instance started on the last VM.
  nkvd=0
  redisaddr="10.0.76.3:6379"
  n_vm=15
  for nclerk in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/redis/$nclerk
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_kv $n_vm $nkvd $nclerk $auto $redisaddr $perf_dir
  done
}

kv_elasticity() {
  auto="auto"
  nkvd=1
  nclerk=16
  redisaddr=""
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  run_kv $n_vm $nkvd $nclerk $auto "$redisaddr" $perf_dir
}

realm_burst() {
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run RealmBurst > /tmp/bench.out 2>&1
  "
  run_benchmark $n_vm $perf_dir "$cmd"
}

realm_balance() {
  mrapp=mr-grep-wiki.yml
  nclerk=8
  clerk_dur="120s"
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    $PRIVILEGED_BIN/realm/create $REALM2; \
    go clean -testcache; \
    go test -v ulambda/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 --realm2 $REALM2 -run RealmBalance --nclerk $nclerk --clerk_dur $clerk_dur --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $n_vm $perf_dir "$cmd"
}

# ========== Make Graphs ==========

graph_mr_aggregate_tpt() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/mr_scalability/16 --out $GRAPH_OUT_DIR/$graph.pdf --title "MapReduce Aggregate Throughput"
}

graph_mr_scalability() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/scalability.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --units "usec" --xlabel "Number of VMs" --ylabel "Speedup Over 1VM" --title "MapReduce End-to-end Speedup" --speedup
}

graph_mr_vs_corral() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  echo "TODO"
  # TODO
}

graph_mr_overlap() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  echo "TODO"
  # TODO
}

graph_kv_aggregate_tpt() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/kv_scalability/16 --out $GRAPH_OUT_DIR/$graph.pdf --title "16 Clerks' Aggregate Throughput Accessing 1 KV Server"
}

graph_kv_scalability() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/scalability.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --units "ops/sec" --xlabel "Number of Clerks" --ylabel "Aggregate Throughput (ops/sec)" --title "Single Key-Value Server Throughput"
}

graph_kv_elasticity() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --title "Throughput of a Dynamically-Scaled KV Service with 16 Clerks"
}

graph_realm_burst() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  echo "TODO"
  # TODO
}

graph_realm_balance() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM1 --kv_realm $REALM2 --title "Aggregate Throughput Balancing 2 Realms' Applications"
}

# ========== Run benchmarks ==========
#mr_scalability
mr_vs_corral
#mr_overlap
#kv_scalability
#kv_elasticity
#realm_burst
#realm_balance

# ========== Produce graphs ==========
source ~/env/3.10/bin/activate
#graph_mr_aggregate_tpt
#graph_mr_scalability
#graph_mr_vs_corral
#graph_mr_overlap
#graph_kv_aggregate_tpt
#graph_kv_scalability
#graph_kv_elasticity
#graph_realm_burst
#graph_realm_balance

echo -e "\n\n\n\n===================="
echo "Results in $OUT_DIR"
echo "Graphs in $GRAPH_OUT_DIR"
