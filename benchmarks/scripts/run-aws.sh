#!/bin/bash

usage() {
  echo "Usage: $0 --vpc VPC --kvpc KVPC --tag TAG --version VERSION [--cloudlab" 1>&2
}

VPC=""
KVPC=""
CLOUDLAB=""
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
  --cloudlab)
    shift
    CLOUDLAB="true"
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

# REALM1 is always the LC realm, REALM2 is always the BE realm.
REALM1="benchrealm1"
REALM2="benchrealm2"

# Set some variables
ROOT_DIR=$(realpath $(dirname $0)/../..)
AWS_DIR=$ROOT_DIR/aws
CLOUDLAB_DIR=$ROOT_DIR/cloudlab
OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs
INIT_OUT=/tmp/init.out

SCRIPT_DIR=$AWS_DIR
if ! [ -z "$CLOUDLAB" ]; then
  SCRIPT_DIR=$CLOUDLAB_DIR
fi

# Get the private IP address of the leader.
cd $SCRIPT_DIR
LEADER_IP_SIGMA=$(./leader-ip.sh --vpc $VPC)
LEADER_IP_K8S=$(./leader-ip.sh --vpc $KVPC)
LEADER_IP=$LEADER_IP_SIGMA

# cd to the sigmaos root directory
cd $ROOT_DIR
mkdir $OUT_DIR

# ========== Helpers ==========

start_cluster() {
  if [ $# -ne 4 ]; then
    echo "start_cluster args: vpc n_cores n_vm swap" 1>&2
    exit 1
  fi
  vpc=$1
  n_cores=$2
  n_vm=$3
  swap=$4
  cd $SCRIPT_DIR
  echo "" > $INIT_OUT
  if [[ $swap == "swapon" ]]; then
    # Enable 16GiB of swap.
    ./setup-swap.sh --vpc $vpc --parallel --n 16777216 >> $INIT_OUT 2>&1
  else
    ./setup-swap.sh --vpc $vpc --parallel >> $INIT_OUT 2>&1
  fi
  ./stop-sigmaos.sh --vpc $vpc --parallel >> $INIT_OUT 2>&1
  ./start-sigmaos.sh --vpc $vpc --ncores $n_cores --n $n_vm --pull $TAG >> $INIT_OUT 2>&1
  cd $ROOT_DIR
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
  cd $SCRIPT_DIR
  ./collect-results.sh --vpc $vpc --perfdir $perf_dir --parallel >> $INIT_OUT 2>&1
  cd $ROOT_DIR
}

run_benchmark() {
  if [ $# -ne 9 ]; then
    echo "run_benchmark args: vpc n_cores n_vm perf_dir cmd vm is_driver async swap" 1>&2
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
  swap=$9
  # Start the cluster if this is the benchmark driver.
  if [[ $is_driver == "true" ]]; then
    # Avoid doing duplicate work.
    if ! should_skip $perf_dir true ; then
      return 0
    fi
    start_cluster $vpc $n_cores $n_vm $swap
  fi
  cd $SCRIPT_DIR
  # Start the benchmark as a background task.
  ./run-benchmark.sh --vpc $vpc --command "$cmd" --vm $vm &
  cd $ROOT_DIR
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
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false "swapoff"
}

run_hotel() {
  if [ $# -ne 12 ]; then
    echo "run_hotel args: testname rps cli_vm nclnt cache_type k8saddr dur sleep autoscale_cache perf_dir driver async" 1>&2
    exit 1
  fi
  testname=$1
  rps=$2
  cli_vm=$3
  nclnt=$4
  cache_type=$5
  k8saddr=$6
  dur=$7
  slp=$8
  autoscale_cache=$9
  perf_dir=${10}
  driver=${11}
  async=${12}
  hotel_ncache=3
  hotel_cache_ncore=2
  as_cache=""
  if [[ $autoscale_cache == "true" ]]; then
     as_cache="--hotel_cache_autoscale"
  fi
  cmd="
    aws s3 rm --profile me-mit --recursive s3://9ps3/hotelperf/k8s > /dev/null; \
    export SIGMADEBUG=\"TEST;THROUGHPUT;CPU_UTIL;\"; \
    go clean -testcache; \
    ulimit -n 100000; \
    go test -v sigmaos/benchmarks -timeout 0 --run $testname --rootNamedIP $LEADER_IP --k8saddr $k8saddr --nclnt $nclnt --hotel_ncache $hotel_ncache --cache_type $cache_type --hotel_cache_ncore $hotel_cache_ncore $as_cache --hotel_dur $dur --hotel_max_rps $rps --sleep $slp --prewarm_realm --memcached '10.0.169.210:11211,10.0.57.124:11211,10.0.91.157:11211'  > /tmp/bench.out 2>&1
  "
#    go test -v sigmaos/benchmarks -timeout 0 --run $testname --rootNamedIP $LEADER_IP --k8saddr $k8saddr --nclnt $nclnt --hotel_ncache $hotel_ncache --cache_type $cache_type --hotel_cache_ncore $hotel_cache_ncore --hotel_dur 60s --hotel_max_rps $rps --prewarm_realm > /tmp/bench.out 2>&1
  if [ "$sys" = "Sigmaos" ]; then
    vpc=$VPC
  else
    # If running against k8s, pass through k8s VPC
    vpc=$KVPC
  fi
  n_cores=4
  run_benchmark $vpc $n_cores 8 $perf_dir "$cmd" $cli_vm $driver $async "swapoff"
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
  run_benchmark $vpc $n_cores 8 $perf_dir "$cmd" $cli_vm $driver $async "swapoff"
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
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false "swapoff"
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
  run_benchmark $VPC $n_cores $n_vm $perf_dir "$cmd" 0 true false "swapoff"
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
  k8saddr="$(cd $SCRIPT_DIR; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  for sys in Sigmaos K8s ; do
    testname="Hotel${sys}Search"
    if [ "$sys" = "Sigmaos" ]; then
      cli_vm=8
      vpc=$VPC
      LEADER_IP=$LEADER_IP_SIGMA
    else
      cli_vm=8
      vpc=$KVPC
      LEADER_IP=$LEADER_IP_K8S
    fi
    # for rps in 100 250 500 1000 1500 2000 2500 3000 3500 4000 4500 5000 5500 6000 6500 7000 7500 8000 ; do
    for rps in 1000 3000 ; do #3000 ; do
      run=${FUNCNAME[0]}/$sys/$rps
      echo "========== Running $run =========="
      perf_dir=$OUT_DIR/$run
      run_hotel $testname $rps $cli_vm 1 "cached" $k8saddr "60s" "0s" false $perf_dir true false
#      run_hotel $testname $rps $cli_vm 1 "memcached" $k8saddr "60s" $perf_dir true false
    done
  done
}

hotel_tail_reserve() {
  for sys in Sigmaos ; do
    testname="Hotel${sys}Reserve"
    cli_vm=8
    vpc=$VPC
    LEADER_IP=$LEADER_IP_SIGMA
    # for rps in 100 250 500 1000 1500 2000 2500 3000 3500 4000 4500 5000 5500 6000 6500 7000 7500 8000 ; do
    for rps in 1000 ; do
      run=${FUNCNAME[0]}/$sys/$rps
      echo "========== Running $run =========="
      perf_dir=$OUT_DIR/$run
      run_hotel $testname $rps $cli_vm 1 "cached" "x.x.x.x" "60s" "0s" false $perf_dir true false
#      run_hotel $testname $rps $cli_vm 1 "memcached" "x.x.x.x" "60s" $perf_dir true false
    done
  done
}

hotel_tail_multi() {
  k8saddr="x.x.x.x"
#  rps="250,500,1000,2000,1000"
#  dur="10s,20s,20s,20s,10s"
  rps="250,500,1000,1500,1000"
  dur="30s,30s,30s,30s,30s"
#  rps="251"
#  dur="10s"
#  sys="Sigmaos"
  sys="K8s"
  cache_type="cached"
  scale_cache="false"
  cache_type2="cached"
#  cache_type="kvd"
  n_clnt_vms=3
  driver_vm=8
  clnt_vma=($(echo "$driver_vm 9 10 11 12 13 14"))
  clnt_vms=${clnt_vma[@]:0:$n_clnt_vms}
  testname_driver="Hotel${sys}Search"
  testname_clnt="Hotel${sys}JustCliSearch"
  pn=""
  if [[ $scale_cache == "true" ]]; then
    pn="-scalecache-true"
  fi
  if [[ "$sys" != "K8s" ]]; then
    cache_type2=""
  fi
  run=${FUNCNAME[0]}/$sys/"rps-$rps-nclnt-$n_clnt_vms$pn$cache_type2"
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/"$run"
  # Avoid doing duplicate work.
  if ! should_skip $perf_dir false ; then
    return
  fi
  if [[ "$sys" == "Sigmaos" ]]; then
    vpc=$VPC
    LEADER_IP=$LEADER_IP_SIGMA
  else
    vpc=$KVPC
    LEADER_IP=$LEADER_IP_K8S
    cd $SCRIPT_DIR
    echo "Stopping hotel"
    ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
    ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
    ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
    echo "Stopping mr"
    ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
    sleep 10
    # Start Hotel
    echo "Starting hotel"
#    ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes --nrunning 19
    ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-$cache_type2 --nrunning 3
    ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes --nrunning 19
    cd $ROOT_DIR
    k8saddr="$(cd $SCRIPT_DIR; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  fi
  for cli_vm in $clnt_vms ; do
    driver="false"
    if [[ $cli_vm == $driver_vm ]]; then
      testname=$testname_driver
      driver="true"
    else
      testname=$testname_clnt
    fi
    run_hotel $testname $rps $cli_vm $n_clnt_vms $cache_type $k8saddr $dur "0s" "$scale_cache" $perf_dir $driver true
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
  cd $SCRIPT_DIR
  echo "Stopping hotel"
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
  # Stop MR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  cd $ROOT_DIR
}

realm_balance_be() {
#  mrapp=mr-wc-wiki4G.yml
#  hotel_dur="20s,20s,20s"
  mrapp=mr-grep-wiki20G.yml
  sl="40s"
  n_vm=8
  n_realm=4
  driver_vm=8
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;BENCH;\"; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBalanceMRMR --rootNamedIP $LEADER_IP --sleep $sl --mrapp $mrapp --nrealm $n_realm > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false "swapoff"
}

k8s_balance_be() {
#  mrapp=mr-wc-wiki4G.yml
#  hotel_dur="20s,20s,20s"
  sl="40s"
  n_vm=8
  # Config
  n_realm=4
  driver_vm=8
  s3dir="corralperf/k8s"
  k8sleaderip=$LEADER_IP_K8S
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  if ! should_skip $perf_dir false ; then
    return
  fi
  cd $SCRIPT_DIR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  sleep 10
  cd $ROOT_DIR
  npods=(17 25 26 27)
  for (( i=1; i<=$n_realm; i++)) do
    idx=$((i - 1))
    np="${npods[$idx]}"
    # Start MR
    echo "Starting mr $i"
    # Remove old results
    aws s3 rm --profile me-mit --recursive s3://9ps3/$s3dir-$i > /dev/null; \
    cd $SCRIPT_DIR
    ./start-k8s-app.sh --vpc $KVPC --path "corral/k8s20G-$i" --nrunning $np
    cd $ROOT_DIR
    sleep 5s
  done
  cmd="
    export SIGMADEBUG=\"TEST;BENCH;\"; \
    echo done removing ; \
    go clean -testcache; \
    echo get ready to run ; \
    go test -v sigmaos/benchmarks -timeout 0 --run K8sMRMulti --rootNamedIP $k8sleaderip --k8sleaderip $k8sleaderip --s3resdir $s3dir --sleep $sl --nrealm $n_realm > /tmp/bench.out 2>&1
  "
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false "swapoff"
}

realm_balance() {
#  mrapp=mr-wc-wiki4G.yml
#  hotel_dur="20s,20s,20s"
  mrapp=mr-grep-wiki20G.yml
  hotel_dur="20s,20s,40s"
  hotel_max_rps="1000,3000,1000"
  hotel_ncache=3
  sl="20s"
  n_vm=8
  driver_vm=8
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  cmd="
    export SIGMADEBUG=\"TEST;BENCH;CPU_UTIL;\"; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBalanceMRHotel --rootNamedIP $LEADER_IP --sleep $sl --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --hotel_ncache $hotel_ncache --mrapp $mrapp > /tmp/bench.out 2>&1
  "
  run_benchmark $VPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false "swapoff"
}

realm_balance_multi() {
  mrapp=mr-grep-wiki20G.yml
  hotel_dur="5s,5s,10s,15s,20s,15s"
  hotel_max_rps="250,500,1000,1500,2000,1000"
  mem_pressure="false"
  hotel_ncache=3
  sl="10s"
  n_vm=8
  driver_vm=8
  sl2="10s"
### Hotel client params
  n_clnt_vms=4
  sys="Sigmaos"
  cache_type="cached"
  clnt_vma=($(echo "$driver_vm 9 10 11 12 13 14"))
  clnt_vms=${clnt_vma[@]:0:$n_clnt_vms}
  testname_clnt="HotelSigmaosJustCliSearch"
  LEADER_IP=$LEADER_IP_SIGMA
  vpc=$VPC
  k8saddr="x.x.x.x"
###
  swap="swapoff"
  bmem=""
  memp=""
  if [[ $mem_pressure == "true" ]]; then
    memp="_mempressure"
    swap="swapon"
    bmem="--block_mem 12GiB"
    sl2="90s"
  fi
  run=${FUNCNAME[0]}$memp
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  # Avoid doing duplicate work.
  if ! should_skip $perf_dir false ; then
    return
  fi
  cmd="
    export SIGMADEBUG=\"TEST;BENCH;CPU_UTIL;UPROCDMGR;\"; \
    go clean -testcache; \
    go test -v sigmaos/benchmarks -timeout 0 --run RealmBalanceMRHotel --rootNamedIP $LEADER_IP --sleep $sl --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --hotel_ncache $hotel_ncache --mrapp $mrapp $bmem --nclnt $n_clnt_vms > /tmp/bench.out 2>&1
  "
  # Start driver VM asynchronously.
  run_benchmark $VPC 4 $n_vm $perf_dir "$cmd" $driver_vm true true $swap
  sleep $sl2
  for cli_vm in $clnt_vms ; do
    driver="false"
    if [[ $cli_vm == $driver_vm ]]; then
      # Already started above.
      continue
    else
      testname=$testname_clnt
    fi
    run_hotel $testname $hotel_max_rps $cli_vm $n_clnt_vms $cache_type $k8saddr $hotel_dur $sl false $perf_dir $driver true
  done
  # Wait for all clients to terminate.
  wait
  end_benchmark $vpc $perf_dir
}

k8s_balance() {
  k8sleaderip=$LEADER_IP_K8S
  hotel_dur="40s,20s,50s"
  hotel_max_rps="1000,3000,1000"
  s3dir="corralperf/k8s"
  n_vm=1
  driver_vm=8
  run=${FUNCNAME[0]}
  perf_dir=$OUT_DIR/$run
  echo "========== Running $run =========="
  # Avoid doing duplicate work.
  if ! should_skip $perf_dir false ; then
    return
  fi
  # Stop Hotel
  cd $SCRIPT_DIR
  echo "Stopping hotel"
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
  # Stop MR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  sleep 10
  # Start Hotel
  echo "Starting hotel"
  ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached --nrunning 3
  ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes --nrunning 19
  # Start MR
  echo "Starting mr"
  ./start-k8s-app.sh --vpc $KVPC --path corral/k8s20G --nrunning 24
  cd $ROOT_DIR
  k8saddr="$(cd $SCRIPT_DIR; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/$s3dir > /dev/null; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/hotelperf/k8s > /dev/null; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/ouptut > /dev/null; \
    echo done removing ; \
    go clean -testcache; \
    echo get ready to run ; \
    go test -v sigmaos/benchmarks -timeout 0 --run K8sBalanceHotelMR --rootNamedIP $k8sleaderip --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --k8sleaderip $k8sleaderip --k8saddr $k8saddr --s3resdir $s3dir > /tmp/bench.out 2>&1
  "
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false "swapoff"
  cd $SCRIPT_DIR
  echo "Stopping hotel"
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
  # Stop MR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  cd $ROOT_DIR
}

k8s_balance_multi() {
  k8sleaderip=$LEADER_IP_K8S
  hotel_dur="5s,5s,10s,15s,20s,15s"
  hotel_max_rps="250,500,1000,1500,2000,1000"
  ### Hotel cli params
  n_clnt_vms=4
  sys="K8s"
  cache_type="cached"
  clnt_vma=($(echo "$driver_vm 9 10 11 12 13 14"))
  clnt_vms=${clnt_vma[@]:0:$n_clnt_vms}
  testname_clnt="Hotel${sys}JustCliSearch"
  LEADER_IP=$LEADER_IP_K8S
  vpc=$VPC
  ### 
  sl="10s"
  sl2="10s"
  s3dir="corralperf/k8s"
  n_vm=1
  driver_vm=8
  run=${FUNCNAME[0]}
  echo "========== Running $run =========="
  perf_dir=$OUT_DIR/$run
  # Avoid doing duplicate work.
  if ! should_skip $perf_dir false ; then
    return
  fi
  # Stop Hotel
  cd $SCRIPT_DIR
  echo "Stopping hotel"
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
  # Stop MR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  sleep 10
  # Start Hotel
  echo "Starting hotel"
  ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached --nrunning 3
  ./start-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes --nrunning 19
  # Start MR
  echo "Starting mr"
  ./start-k8s-app.sh --vpc $KVPC --path corral/k8s20G --nrunning 24
  cd $ROOT_DIR
  k8saddr="$(cd $SCRIPT_DIR; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
  cmd="
    export SIGMADEBUG=\"TEST;\"; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/$s3dir > /dev/null; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/hotelperf/k8s > /dev/null; \
    aws s3 rm --profile me-mit --recursive s3://9ps3/ouptut > /dev/null; \
    echo done removing ; \
    go clean -testcache; \
    echo get ready to run ; \
    go test -v sigmaos/benchmarks -timeout 0 --run K8sBalanceHotelMR --rootNamedIP $k8sleaderip --hotel_dur $hotel_dur --hotel_max_rps $hotel_max_rps --k8sleaderip $k8sleaderip --k8saddr $k8saddr --s3resdir $s3dir --nclnt $n_clnt_vms > /tmp/bench.out 2>&1
  "
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true true "swapoff"
  sleep $sl2
  for cli_vm in $clnt_vms ; do
    driver="false"
    if [[ $cli_vm == $driver_vm ]]; then
      # Already started above.
      continue
    else
      testname=$testname_clnt
    fi
    run_hotel $testname $hotel_max_rps $cli_vm $n_clnt_vms $cache_type $k8saddr $hotel_dur $sl false $perf_dir $driver true
  done
  # Wait for all clients to terminate.
  wait
  end_benchmark $vpc $perf_dir
  cd $SCRIPT_DIR
  echo "Stopping hotel"
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-cached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes-memcached
  ./stop-k8s-app.sh --vpc $KVPC --path DeathStarBench/hotelReservation/kubernetes
  # Stop MR
  echo "Stopping mr"
  ./stop-k8s-app.sh --vpc $KVPC --path "corral/"
  cd $ROOT_DIR
}

mr_k8s() {
  n_vm=1
  k8saddr="$(cd $SCRIPT_DIR; ./get-k8s-svc-addr.sh --vpc $KVPC --svc frontend):5000"
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
  run_benchmark $KVPC 4 $n_vm $perf_dir "$cmd" $driver_vm true false "swapoff"
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
  run_benchmark $VPC $n_vm $perf_dir "$cmd" 0 true false "swapoff"
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

graph_hotel_tail_tpt_over_time() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  d="hotel_tail_multi/Sigmaos/rps-250,500,1000,2000,1000-nclnt-4"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$d --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm "" --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Hotel Latency Under Changing Load $d" --total_ncore 32
}

graph_k8s_hotel_tail_tpt_over_time() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
#  d="hotel_tail_multi/K8s/rps-250,500,1000,1000,1000-nclnt-3cached"
  d="hotel_tail_multi/K8s/rps-250,500,1000,1500,1000-nclnt-3cached"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$d --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm "" --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Hotel Latency Under Changing Load $d" --total_ncore 32
}

graph_hotel_tail_tpt_over_time_autoscale() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  d="hotel_tail_multi/Sigmaos/rps-250,500,1000,2000,1000-nclnt-4-scalecache-true"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$d --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm "" --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Hotel Latency Under Changing Load $d" --total_ncore 32
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
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM1 --hotel_realm $REALM2 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32
}

graph_realm_balance_multi() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM2 --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32
}

graph_realm_balance_multi_mempressure() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM2 --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32
}

graph_realm_balance_be() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  nrealm=4
  $GRAPH_SCRIPTS_DIR/mrmr-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --nrealm $nrealm --units "MB/sec" --title "Aggregate Throughput Balancing $nrealm Realms' BE Applications" --total_ncore 32
}

graph_k8s_balance_be() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  nrealm=4
  $GRAPH_SCRIPTS_DIR/mrmr-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --nrealm $nrealm --units "MB/sec" --title "Aggregate Throughput Balancing $nrealm Realms' BE Applications" --total_ncore 32
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
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM2 --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --k8s # --xmin 200000 --xmax 400000
}

graph_k8s_balance_multi() {
  fname=${FUNCNAME[0]}
  graph="${fname##graph_}"
  echo "========== Graphing $graph =========="
  $GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $OUT_DIR/$graph --out $GRAPH_OUT_DIR/$graph.pdf --mr_realm $REALM2 --hotel_realm $REALM1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --k8s # --xmin 200000 --xmax 400000
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
hotel_tail_multi
#realm_balance_be
#realm_balance_multi
#k8s_balance_multi
#k8s_balance_be
# XXX Try above next
#k8s_balance
# XXX
#realm_balance
#hotel_tail
#hotel_tail_reserve
#mr_replicated_named
#realm_burst
#kv_vs_cached
#mr_vs_corral
#rpcbench_tail_multi
# XXX mr_scalability
#mr_k8s

# ========== Produce graphs ==========
source ~/env/3.10/bin/activate
#graph_realm_balance_be
#graph_k8s_balance_be
#graph_realm_balance_multi
#graph_k8s_balance_multi
graph_k8s_hotel_tail_tpt_over_time




#graph_hotel_tail_tpt_over_time
#graph_hotel_tail_tpt_over_time_autoscale
# XXX
#graph_k8s_balance
#graph_realm_balance
#graph_mr_replicated_named
#graph_mr_vs_corral
# XXX graph_mr_aggregate_tpt
# XXX graph_mr_scalability
#graph_k8s_mr_aggregate_tpt
#scrape_realm_burst
#graph_hotel_tail

echo -e "\n\n\n\n===================="
echo "Results in $OUT_DIR"
echo "Graphs in $GRAPH_OUT_DIR"
