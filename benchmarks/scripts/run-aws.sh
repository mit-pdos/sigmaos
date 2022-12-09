#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --kvpc KVPC --realm1 REALM1 --realm2 REALM2 [--version VERSION]" 1>&2
}

VPC=""
KVPC=""
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
  --kvpc)
    shift
    KVPC="$1"
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

if [ -z "$VPC" ] || [ -z "$KVPC" ] || [ -z "$REALM1" ] || [ -z "$REALM2" ] || [ -z "$VERSION" ] || [ $# -gt 0 ]; then
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
  if [ $# -ne 3 ]; then
    echo "start_cluster args: vpc n_cores n_vm" 1>&2
    exit 1
  fi
  vpc=$1
  n_cores=$2
  n_vm=$3
  cd $AWS_DIR
  echo "" > $INIT_OUT
  ./stop-sigmaos.sh --vpc $vpc --parallel >> $INIT_OUT 2>&1
  ./build-sigma.sh --vpc $vpc --realm $REALM1 --version $VERSION --branch master >> $INIT_OUT 2>&1
  ./build-sigma.sh --vpc $vpc --realm $REALM2 --version $VERSION --branch master >> $INIT_OUT 2>&1
  ./install-sigma.sh --vpc $vpc --realm $REALM1 --parallel >> $INIT_OUT 2>&1
  ./install-sigma.sh --vpc $vpc --realm $REALM2 --parallel >> $INIT_OUT 2>&1
  ./start-sigmaos.sh --vpc $vpc --realm $REALM1 --ncores $n_cores --n $n_vm >> $INIT_OUT 2>&1
  cd $DIR
}

run_benchmark() {
  if [ $# -ne 6 ]; then
    echo "run_benchmark args: vpc n_cores n_vm perfdir cmd vm" 1>&2
    exit 1
  fi
  vpc=$1
  n_cores=$2
  n_vm=$3
  perf_dir=$4
  cmd=$5
  vm=$6 # benchmark driver vm index (usually 0)
  # Avoid doing duplicate work.
  if [ -d $perf_dir ]; then
    benchname="${perf_dir#$OUT_DIR/}"
    echo "========== Skipping $benchname (already ran) =========="
    return 0
  fi
  start_cluster $vpc $n_cores $n_vm
  mkdir -p $perf_dir
  cd $AWS_DIR
  ./run-benchmark.sh --vpc $vpc --command "$cmd" --vm $vm
  ./collect-results.sh --vpc $vpc --perfdir $perf_dir --parallel >> $INIT_OUT 2>&1
  cd $DIR
}

run_mr() {
  if [ $# -ne 4 ]; then
    echo "run_mr args: n_cores n_vm app perf_dir" 1>&2
    exit 1
  fi
  n_cores=$1
  n_vm=$2
  mrapp=$3
  perf_dir=$4
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppMR --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0
}

run_hotel() {
  if [ $# -ne 4 ]; then
    echo "run_hotel args: sys rps k8saddr perf_dir" 1>&2
    exit 1
  fi
  sys=$1
  rps=$2
  k8saddr=$3
  perf_dir=$4
  cmd="
    go clean -testcache; \
    ulimit -n 100000; \
    go test -v sigmaos/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run Hotel${sys}Search --k8saddr $k8saddr --hotel_dur 60s --hotel_max_rps $rps --pregrow_realm > /tmp/bench.out 2>&1
  "
  if [ "$sys" = "Sigmaos" ]; then
    vpc=$VPC
    # We're only running on 5 machines, so use #14 as a client (it is unused).
    cli_vm=14
  else
    # If running against k8s, pass through k8s VPC
    vpc=$KVPC
    # If running against k8s, we make sure not to untaint the control node, so
    # node 0 can't run any pods. We then use this as the client machine. Thus,
    # we should make sure to start up an extra node in the k8s cluster.
    cli_vm=0
  fi
  n_cores=4
  # Since we only use 5 VMs anyway, it should be fine to run the client on VM 14, which should also exist in the k8s cluster.
  run_benchmark $vpc $n_cores 8 $perf_dir "$cmd" $cli_vm
}

#run_kv() {
#  if [ $# -ne 7 ]; then
#    echo "run_kv args: n_vm nkvd kvd_ncore nclerk auto redisaddr perf_dir" 1>&2
#    exit 1
#  fi
#  n_vm=$1
#  nkvd=$2
#  nkvd_ncore=$3
#  nclerk=$4
#  auto=$5
#  redisaddr=$6
#  perf_dir=$7
#  cmd="
#    go clean -testcache; \
#    go test -v sigmaos/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run AppKVUnrepl --nkvd $nkvd --kvd_ncore $kvd_ncore --nclerk $nclerk --kvauto $auto --redisaddr \"$redisaddr\" > /tmp/bench.out 2>&1
#  "
#  run_benchmark $VPC $n_vm $perf_dir "$cmd"
#}

# ========== Top-level benchmarks ==========

mr_scalability() {
  mrapp=mr-grep-wiki-120G.yml
  for n_vm in 1 2 4 8 16 ; do
    run=${FUNCNAME[0]}/sigmaOS/$n_vm
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr 4 $n_vm $mrapp $perf_dir
  done
}

mr_vs_corral() {
  n_vm=8
  app="mr-wc-wiki"
  dataset_size="2G"
  for size in $dataset_size ; do
    mrapp="$app$size.yml"
    run=${FUNCNAME[0]}/$mrapp
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr 2 $n_vm $mrapp $perf_dir
  done
}

hotel_tail() {
  # Make sure to fill in new k8s addr.
  k8saddr="10.111.131.148:5000"
  for sys in Sigmaos K8s ; do
    for rps in 100 250 500 1000 1500 2000 2500 3000 3500 4000 4500 5000 5500 6000 6500 7000 7500 8000 ; do
      run=${FUNCNAME[0]}/$sys/$rps
      echo "========== Running $run =========="
      perf_dir=$OUT_DIR/$run
      run_hotel $sys $rps $k8saddr $perf_dir
    done
  done
}

#mr_overlap() {
#  mrapp=mr-wc-wiki4G.yml
#  n_vm=16
#  run=${FUNCNAME[0]}
#  echo "========== Running $run =========="
#  perf_dir=$OUT_DIR/$run
#  # TODO
#  echo "TODO"
##  run_mr $n_vm $mrapp $perf_dir
#}

#kv_scalability() {
#  # First, run against our KV.
#  auto="manual"
#  nkvd=1
#  kvd_ncore=1
#  redisaddr=""
#  n_vm=16
#  for nclerk in 1 2 4 8 16 ; do
#    run=${FUNCNAME[0]}/sigmaOS/$nclerk
#    echo "========== Running $run =========="
#    perf_dir=$OUT_DIR/$run
#    run_kv $n_vm $nkvd $kvd_ncore $nclerk $auto "$redisaddr" $perf_dir
#  done
#
#  # Then, run against a redis instance started on the last VM.
#  nkvd=0
#  redisaddr="10.0.134.192:6379"
#  n_vm=15
#  for nclerk in 1 2 4 8 16 ; do
#    run=${FUNCNAME[0]}/redis/$nclerk
#    echo "========== Running $run =========="
#    perf_dir=$OUT_DIR/$run
#    run_kv $n_vm $nkvd $kvd_ncore $nclerk $auto $redisaddr $perf_dir
#  done
#}

#kv_elasticity() {
#  auto="auto"
#  nkvd=1
#  kvd_ncore=2
#  nclerk=16
#  redisaddr=""
#  n_vm=16
#  run=${FUNCNAME[0]}
#  echo "========== Running $run =========="
#  perf_dir=$OUT_DIR/$run
#  run_kv $n_vm $nkvd $kvd_ncore $nclerk $auto "$redisaddr" $perf_dir
#}

realm_burst() {
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 -run RealmBurst > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_vm $perf_dir "$cmd" 0
}

realm_balance() {
  mrapp=mr-grep-wiki.yml
  hotel_dur="60s"
  hotel_max_rps=1000
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    $PRIVILEGED_BIN/realm/create $REALM2; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --version=$VERSION --realm $REALM1 --realm2 $REALM2 -run RealmBalance --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_vm $perf_dir "$cmd" 0
}

# ========== Make Graphs ==========

graph_mr_aggregate_tpt() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/mr_scalability/sigmaOS/16 --out $GRAPH_OUT_DIR/$graph.pdf --title "MapReduce Aggregate Throughput"
}

graph_mr_scalability() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/scalability.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --xlabel "Number of VMs" --ylabel "Speedup Over 1VM" --title "Elastic MapReduce End-to-end Speedup" --speedup
}

graph_mr_vs_corral() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  datasize=2G
  $GRAPH_SCRIPTS_DIR/mr_vs_corral.py --measurement_dir $OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/$graph.pdf --datasize=$datasize
}

graph_hotel_tail() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/tail_latency.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf 
}

#graph_mr_overlap() {
#  fname=${FUNCNAME[0]}
#  graph="${fname##graph_}"
#  echo "========== Graphing $graph =========="
#  echo "TODO"
#  # TODO
#}

#graph_kv_aggregate_tpt() {
#  fname=${FUNCNAME[0]}
#  graph="${fname##graph_}"
#  echo "========== Graphing $graph =========="
#  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/kv_scalability/sigmaOS/16 --out $GRAPH_OUT_DIR/$graph.pdf --title "16 Clerks' Aggregate Throughput Accessing 1 KV Server"
#}
#
#graph_kv_scalability() {
#  fname=${FUNCNAME[0]}
#  graph="${fname##graph_}"
#  echo "========== Graphing $graph =========="
#  $GRAPH_SCRIPTS_DIR/scalability.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --units "ops/sec" --xlabel "Number of Clerks" --ylabel "Aggregate Throughput (ops/sec)" --title "Single Key-Value Server Throughput"
#}
#
#graph_kv_elasticity() {
#  fname=${FUNCNAME[0]}
#  graph="${fname##graph_}"
#  echo "========== Graphing $graph =========="
#  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --title "Throughput of a Dynamically-Scaled KV Service with 16 Clerks"
#}

scrape_realm_burst() {
  fname=${FUNCNAME[0]}
  graph="${fname##scrape_}"
  echo "========== Scraping $graph =========="
  dir=$OUT_DIR/$graph
  res=$(grep "PASS:" $dir/bench.out)
  echo -e "Result: $res"
}

graph_realm_balance() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM1 --kv_realm $REALM2 --title "Aggregate Throughput Balancing 2 Realms' Applications"
}

# ========== Preamble ==========
echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
echo "Running benchmarks with version: $VERSION"

# ========== Run benchmarks ==========
mr_scalability
#mr_vs_corral
#realm_burst
#realm_balance
hotel_tail

# ========== Produce graphs ==========
source ~/env/3.10/bin/activate
graph_mr_aggregate_tpt
graph_mr_scalability
#graph_mr_vs_corral
#scrape_realm_burst
#graph_realm_balance
graph_hotel_tail

echo -e "\n\n\n\n===================="
echo "Results in $OUT_DIR"
echo "Graphs in $GRAPH_OUT_DIR"
