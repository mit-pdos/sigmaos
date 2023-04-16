#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --kvpc KVPC --tag TAG --version VERSION" 1>&2
}

VPC=""
KVPC=""
TAG=""
VERSION=""
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
  --tag)
    shift
    TAG=$1
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

if [ -z "$VPC" ] || [ -z "$KVPC" ] || [ -z "$TAG" ] || [ -z "$VERSION" ] || [ $# -gt 0 ]; then
    usage
    exit 1
fi

# REALM1 is always the BE realm, REALM2 is always the LC realm.
REALM1="benchrealm1"
REALM2="benchrealm2"

# Set some variables
DIR=$(realpath $(dirname $0)/../..)
AWS_DIR=$DIR/aws
OUT_DIR=$DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$DIR/benchmarks/results/graphs
INIT_OUT=/tmp/init.out

# Get the private IP address of the leader.
cd $AWS_DIR
LEADER_IP_SIGMA=$(./leader-ip.sh --vpc $VPC)
LEADER_IP_K8S=$(./leader-ip.sh --vpc $KVPC)
LEADER_IP=$LEADER_IP_SIGMA

# cd to the sigmaos root directory
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
  ./start-sigmaos.sh --vpc $vpc --ncores $n_cores --n $n_vm --pull $TAG >> $INIT_OUT 2>&1
  cd $DIR
}

should_skip() {
  if [ $# -ne 2 ]; then
    echo "should_skip args: perf_dir make_dir" 1>&2
    exit 1
  fi
  perf_dir=$1
  make_dir=$2
  # Check if the experiment has already been run.
  if [ -d $perf_dir ]; then
    benchname="${perf_dir#$OUT_DIR/}"
    echo "========== Skipping $benchname (already ran) =========="
    return 1
  fi
  if [[ $make_dir == "true" ]]; then
    # Create an output directory for the results.
    mkdir -p $perf_dir
  fi
  return 0
}

end_benchmark() {
  if [ $# -ne 2 ]; then
    echo "end_benchmark_driver args: vpc perf_dir" 1>&2
    exit 1
  fi
  vpc=$1
  perf_dir=$2
  cd $AWS_DIR
  ./collect-results.sh --vpc $vpc --perfdir $perf_dir --parallel >> $INIT_OUT 2>&1
  cd $DIR
}

run_benchmark() {
  if [ $# -ne 8 ]; then
    echo "run_benchmark args: vpc n_cores n_vm perf_dir cmd vm is_driver async" 1>&2
    exit 1
  fi
  vpc=$1
  n_cores=$2
  n_vm=$3
  perf_dir=$4
  cmd=$5
  vm=$6 # benchmark driver vm index (usually 0)
  is_driver=$7
  async=$8
  # Start the cluster if this is the benchmark driver.
  if [[ $is_driver == "true" ]]; then
    # Avoid doing duplicate work.
    if ! should_skip $perf_dir true ; then
      return 0
    fi
    start_cluster $vpc $n_cores $n_vm
  fi
  cd $AWS_DIR
  # Start the benchmark as a background task.
  ./run-benchmark.sh --vpc $vpc --command "$cmd" --vm $vm &
  cd $DIR
  # Wait for it to complete, if this benchmark is being run synchronously.
  if [[ $async == "false" ]] ; then
    wait
    end_benchmark $vpc $perf_dir
  fi
}

run_mr() {
  if [ $# -ne 5 ]; then
    echo "run_mr args: n_cores n_vm prewarm app perf_dir" 1>&2
    exit 1
  fi
  n_cores=$1
  n_vm=$2
  prewarm=$3
  mrapp=$4
  perf_dir=$5
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run AppMR $prewarm --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false
}

run_hotel() {
  if [ $# -ne 9 ]; then
    echo "run_hotel args: testname rps cli_vm nclnt cache_type k8saddr perf_dir driver async " 1>&2
    exit 1
  fi
  testname=$1
  rps=$2
  cli_vm=$3
  nclnt=$4
  cache_type=$5
  k8saddr=$6
  perf_dir=$7
  driver=$8
  async=$9
  hotel_ncache=3
  hotel_cache_ncore=2
  cmd="
    export SIGMADEBUG=\"TEST;THROUGHPUT;CPU_UTIL;\"; \
    go clean -testcache; \
    ulimit -n 100000; \
    go test -v sigmaos/benchmarks -timeout 0 --run $testname --rootNamedIP $LEADER_IP --k8saddr $k8saddr --nclnt $nclnt --hotel_ncache $hotel_ncache --cache_type $cache_type --hotel_cache_ncore $hotel_cache_ncore --hotel_dur 60s --hotel_max_rps $rps --prewarm_realm > /tmp/bench.out 2>&1
  "
  if [ "$sys" = "Sigmaos" ]; then
    vpc=$VPC
  else
    # If running against k8s, pass through k8s VPC
    vpc=$KVPC
  fi
  n_cores=4
  run_benchmark $vpc $n_cores 8 $perf_dir "$cmd" $cli_vm $driver $async
}

run_rpcbench() {
  if [ $# -ne 7 ]; then
    echo "run_rpcbench args: testname rps cli_vm nclnt perf_dir driver async " 1>&2
    exit 1
  fi
  testname=$1
  rps=$2
  cli_vm=$3
  nclnt=$4
  perf_dir=$5
  driver=$6
  async=$7
  cmd="
    export SIGMADEBUG=\"TEST;THROUGHPUT;CPU_UTIL;\"; \
    export SIGMAJAEGERIP=\"$LEADER_IP\"; \
    go clean -testcache; \
    ulimit -n 100000; \
    go test -v sigmaos/benchmarks -timeout 0 --run $testname --rootNamedIP $LEADER_IP --k8saddr $k8saddr --nclnt $nclnt --rpcbench_dur 60s --rpcbench_max_rps $rps --prewarm_realm > /tmp/bench.out 2>&1
  "
  if [ "$sys" = "Sigmaos" ]; then
    vpc=$VPC
  else
    # If running against k8s, pass through k8s VPC
    vpc=$KVPC
  fi
  n_cores=4
  run_benchmark $vpc $n_cores 8 $perf_dir "$cmd" $cli_vm $driver $async
}

run_kv() {
  if [ $# -ne 8 ]; then
    echo "run_kv args: n_cores n_vm nkvd kvd_ncore nclerk auto redisaddr perf_dir" 1>&2
    exit 1
  fi
  n_cores=$1
  n_vm=$2
  nkvd=$3
  nkvd_ncore=$4
  nclerk=$5
  auto=$6
  redisaddr=$7
  perf_dir=$8
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 -run AppKVUnrepl --nkvd $nkvd --kvd_ncore $kvd_ncore --nclerk $nclerk --kvauto $auto --redisaddr \"$redisaddr\" > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false
}

run_cached() {
  if [ $# -ne 6 ]; then
    echo "run_cached args: n_cores n_vm nkvd kvd_ncore nclerk perf_dir" 1>&2
    exit 1
  fi
  n_cores=$1
  n_vm=$2
  nkvd=$3
  nkvd_ncore=$4
  nclerk=$5
  perf_dir=$6
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 -run AppCached --nkvd $nkvd --kvd_ncore $kvd_ncore --nclerk $nclerk > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false
}

# ========== Top-level benchmarks ==========

mr_scalability() {
  mrapp=mr-grep-wiki120G.yml
  for n_vm in 1 16 ; do # 2 4 8 
    run=${FUNCNAME[0]}/sigmaOS/$n_vm
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr 4 $n_vm "" $mrapp $perf_dir
  done
}

mr_replicated_named() {
  mrapp=mr-grep-wiki120G.yml
  n_vm=16
  run=${FUNCNAME[0]}/sigmaOS
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  run_mr 4 $n_vm "" $mrapp $perf_dir
}

mr_vs_corral() {
  n_vm=8
  app="mr-wc-wiki"
  dataset_size="2G"
  for prewarm in "" "--prewarm_realm" ; do
    mrapp="$app$dataset_size.yml"
    if [ -z "$prewarm" ]; then
      runname="$mrapp-cold"
    else
      runname="$mrapp-warm"
    fi
    run=${FUNCNAME[0]}/$runname
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/$run
    run_mr 2 $n_vm "$prewarm" $mrapp $perf_dir
  done
}

hotel_tail() {
  k8saddr="$(cd aws; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  echo "Using k8s frontend addr $k8saddr"
  for sys in Sigmaos ; do #K8s ; do
    testname="Hotel${sys}Search"
    if [ "$sys" = "Sigmaos" ]; then
      cli_vm=9
    else
      cli_vm=0
    fi
    # for rps in 100 250 500 1000 1500 2000 2500 3000 3500 4000 4500 5000 5500 6000 6500 7000 7500 8000 ; do
    for rps in 1000 ; do
      run=${FUNCNAME[0]}/$sys/$rps
      echo "========== Running $run =========="
      perf_dir=$OUT_DIR/$run
      run_hotel $testname $rps $cli_vm 1 "cached" $k8saddr $perf_dir true false
    done
  done
}

rpcbench_tail_multi() {
  k8saddr="$(cd aws; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  echo "Using k8s frontend addr $k8saddr"
  rps=2500
  sys="Sigmaos"
#  sys="K8s"
#  cache_type="kvd"
  driver_vm=8
  testname_driver="RPCBench${sys}Sleep"
  testname_clnt="RPCBench${sys}JustCliSleep"
  for n in {1..1000} ; do
    # run=${FUNCNAME[0]}/$sys/$rps #-$n
    run=${FUNCNAME[0]}/$rps-$n
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/"$run"
    # Avoid doing duplicate work.
    if ! should_skip $perf_dir false ; then
      continue
    fi
    for cli_vm in $driver_vm 9 10 11 12 ; do #11 ; do
      driver="false"
      if [[ $cli_vm == $driver_vm ]]; then
        testname=$testname_driver
        driver="true"
      else
        testname=$testname_clnt
      fi
      run_rpcbench $testname $rps $cli_vm 5 $perf_dir $driver true
      if [[ $cli_vm == $driver_vm ]]; then
        # Give the driver time to start up the realm.
        sleep 10s
      fi
    done
    # Wait for all clients to terminate.
    wait
    # Copy results.
    end_benchmark $VPC $perf_dir
    echo "!!!!!!!!!!!!!!!!!! Benchmark done! !!!!!!!!!!!!!!!!!"
    if grep -r "file not found http" $perf_dir ; then
      echo "+++++++++++++++++++ Benchmark failed unexpectedly! +++++++++++++++++++" 
      continue
    fi
    if grep -r "server-side" $perf_dir ; then
      if grep -r "concurrent map writes" /tmp/*.out ; then
        echo "----------------- Error concurrent map writes -----------------"
        continue
      fi
      echo "+++++++++++++++++++ Benchmark successful! +++++++++++++++++++" 
      return
    fi
  done
}


hotel_tail_multi() {
  k8saddr="$(cd aws; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  echo "Using k8s frontend addr $k8saddr"
  rps=2500
#  sys="Sigmaos"
  sys="K8s"
  cache_type="cached"
#  cache_type="kvd"
  driver_vm=8
  testname_driver="Hotel${sys}Search"
  testname_clnt="Hotel${sys}JustCliSearch"
  for n in {1..1} ; do
    if [[ "$sys" == "Sigmaos" ]]; then
      vpc=$VPC
      LEADER_IP=$LEADER_IP_SIGMA
    else
      vpc=$KVPC
      LEADER_IP=$LEADER_IP_K8S
    fi
    # run=${FUNCNAME[0]}/$sys/$rps #-$n
    run=${FUNCNAME[0]}/$sys/ncache-3/$rps-$n
    echo "========== Running $run =========="
    perf_dir=$OUT_DIR/"$run"
    # Avoid doing duplicate work.
    if ! should_skip $perf_dir false ; then
      continue
    fi
    for cli_vm in $driver_vm ; do #9 10 ; do #11 12 ; do
      driver="false"
      if [[ $cli_vm == $driver_vm ]]; then
        testname=$testname_driver
        driver="true"
      else
        testname=$testname_clnt
      fi
      run_hotel $testname $rps $cli_vm 1 $cache_type $k8saddr $perf_dir $driver true
      if [[ $cli_vm == $driver_vm ]]; then
        # Give the driver time to start up the realm.
        sleep 30s
      fi
    done
    # Wait for all clients to terminate.
    wait
    # Copy results.
    end_benchmark $vpc $perf_dir
    echo "!!!!!!!!!!!!!!!!!! Benchmark done! !!!!!!!!!!!!!!!!!"
    if grep -r "file not found http" $perf_dir ; then
      echo "+++++++++++++++++++ Benchmark failed unexpectedly! +++++++++++++++++++" 
      continue
    fi
    if grep -r "concurrent map reads" /tmp/*.out ; then
      echo "----------------- Error concurrent map reads -----------------"
      return
      continue
    fi
    if grep -r "concurrent map writes" /tmp/*.out ; then
      echo "----------------- Error concurrent map writes -----------------"
      return
      continue
    fi
    if grep -r "server-side" $perf_dir ; then
      echo "+++++++++++++++++++ Benchmark successful! +++++++++++++++++++" 
      return
    fi
  done
}

realm_balance() {
#  mrapp=mr-wc-wiki4G.yml
#  hotel_dur="20s,20s,20s"
  mrapp=mr-grep-wiki20G.yml
  hotel_dur="20s,20s,40s"
  hotel_max_rps="1000,3000,1000"
  hotel_ncache=6
  sl="20s"
  n_vm=8
  driver_vm=8
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBalanceMRHotel --rootNamedIP $LEADER_IP --sleep $sl --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --hotel_ncache $hotel_ncache --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false
}

realm_balance_be() {
#  mrapp=mr-wc-wiki4G.yml
#  hotel_dur="20s,20s,20s"
  mrapp=mr-grep-wiki20G.yml
  sl="40s"
  n_vm=8
  driver_vm=8
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBalanceMRMR --rootNamedIP $LEADER_IP --sleep $sl --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false
}

k8s_balance() {
  k8saddr="10.96.81.233:5000"
  k8sleaderip="10.0.134.163"
  hotel_dur="40s,20s,50s"
  hotel_max_rps="1000,3000,1000"
  s3dir="corralperf/k8s"
  n_vm=1
  driver_vm=0
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    aws s3 rm --recursive s3://9ps3/$s3dir > /dev/null; \
    aws s3 rm --recursive s3://9ps3/hotelperf/k8s > /dev/null; \
    aws s3 rm --recursive s3://9ps3/ouptut > /dev/null; \
    echo done removing ; \
    go clean -testcache; \
    echo get ready to run ; \
    go test -v sigmaos/benchmarks -timeout 0 --run K8sBalanceHotelMR --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --k8sleaderip $k8sleaderip --k8saddr $k8saddr --s3resdir $s3dir > /tmp/bench.out 2>&1
  "
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false
}

mr_k8s() {
  n_vm=1
  k8saddr="10.0.134.163"
  s3dir="corralperf/k8s"
  app="mr-k8s-grep"
  run=${FUNCNAME[0]}/$app
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  driver_vm=0
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    aws s3 rm --recursive s3://9ps3/$s3dir > /dev/null; \
    aws s3 rm --recursive s3://9ps3/ouptut > /dev/null; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run MRK8s --k8sleaderip $k8saddr --s3resdir $s3dir > /tmp/bench.out 2>&1
  "
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false
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

kv_vs_cached() {
  # First, run against KVD.
  auto="manual"
  nkvd=1
  nclerk=1
  n_core=4
  kvd_ncore=4
  redisaddr=""
  n_vm=8
  run=${FUNCNAME[0]}/kvd/
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  run_kv $n_core $n_vm $nkvd $kvd_ncore $nclerk $auto "$redisaddr" $perf_dir

  # Then, run against cached.
  nkvd=1
  redisaddr="10.0.134.192:6379"
  n_vm=15
  run=${FUNCNAME[0]}/cached
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  run_cached $n_core $n_vm $nkvd $kvd_ncore $nclerk $perf_dir
}

realm_burst() {
  n_vm=16
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBurst > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC $n_vm $perf_dir "$cmd" 0 true false
}

# ========== Make Graphs ==========

graph_mr_aggregate_tpt() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/mr_scalability/sigmaOS/16 --out $GRAPH_OUT_DIR/$graph.pdf --units "MB/sec" --title "MapReduce Aggregate Throughput" --total_ncore 64
}

graph_mr_replicated_named() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/mr_replicated_named/sigmaOS --out $GRAPH_OUT_DIR/$graph.pdf --units "MB/sec" --title "MapReduce Aggregate Throughput" --total_ncore 64
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
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM1 --hotel_realm $REALM2 --units "99% Lat (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32
}

graph_realm_balance_be() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/mrmr-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --realm1 $REALM1 --realm2 $REALM2 --units "MB/sec" --title "Aggregate Throughput Balancing 2 Realms' BE Applications" --total_ncore 32
}

graph_k8s_mr_aggregate_tpt() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/mr_k8s/mr-k8s-grep/ --out $GRAPH_OUT_DIR/$graph.pdf --units "MB/sec" --title "MapReduce Aggregate Throughput" --total_ncore 64
}

graph_k8s_balance() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM1 --hotel_realm $REALM2 --units "99% Lat (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 0 --k8s --xmin 200000 --xmax 400000
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

# ========== Preamble ==========
echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
echo "Running benchmarks with version: $VERSION"

# ========== Run benchmarks ==========
#mr_replicated_named
#realm_burst
kv_vs_cached
mr_vs_corral
realm_balance
realm_balance_be
hotel_tail
hotel_tail_multi
#rpcbench_tail_multi
# XXX mr_scalability
#mr_k8s
#k8s_balance

# ========== Produce graphs ==========
source ~/env/3.10/bin/activate
#graph_mr_replicated_named
graph_realm_balance_be
graph_realm_balance
graph_mr_vs_corral
#graph_k8s_balance
# XXX graph_mr_aggregate_tpt
# XXX graph_mr_scalability
#graph_k8s_mr_aggregate_tpt
#scrape_realm_burst
#graph_hotel_tail

echo -e "\n\n\n\n===================="
echo "Results in $OUT_DIR"
echo "Graphs in $GRAPH_OUT_DIR"
