#!/bin/bash

usage() {
  echo "Usage: $0 [--sn-only] [--version VERSION] [--pull IMAGE_LABEL]" 1>&2
}

VERSION="XXXX"
PULL="yizhengh"
MR_ARG="--mr_realm benchrealm1"
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
  --pull)
    shift
    PULL=$1
    shift
    ;;
  --version)
    shift
    VERSION=$1
    shift
    ;;
  --sn-only)
    MR_ARG=""
	shift
	;;
  -help)
    usage
    exit 0
    ;;
  *)
    echo "Error: unexpected argument '$1'"
    usage
    exit 1
    ;;
  esac
done

# parameters
sn_dur="5s,5s,10s,5s"
sn_max_rps="600,1200,1800,900"
named_ip="10.10.1.2"
mongo_url="10.10.1.1:4407"
test_driver=11
img_path="1.jpg"
n_resize="1"
n_resize_k8="0"

# directories
ROOT_DIR=$(realpath $(dirname $0)/..)
SCRIPT_DIR=$ROOT_DIR/cloudlab
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

# env variables
source $SCRIPT_DIR/env.sh
source $ROOT_DIR/env/yh_env.sh
vms=`cat servers.txt | cut -d " " -f2`
vma=($vms)
SSHVM="${vma[$test_driver]}"

# ========== Benchmark functions ==========
bench_sigmaos_simple() {
	echo "Running simple sigmaos bench for BE and LC image resizing"
	cd $SCRIPT_DIR
	./stop-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes
	./stop-sigmaos.sh --parallel
	./start-sigmaos.sh --pull $PULL --n 2 --branch social-network-benchmark
	COMMAND="
		go clean -testcache;
		go test -v sigmaos/benchmarks --no-shutdown -timeout 0 --run TestRealmBalanceSimpleImgResize --rootNamedIP $named_ip --imgresize_path $img_path --n_imgresize 5 --n_imgresize_per 1 --imgresize_mcpu 100 &> test_simple.out
	"
	echo "Run [$SSHVM]: $COMMAND"
	ssh -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM <<ENDSSH
  		# Make sure swap is off on the benchmark machines.
  		sudo swapoff -a
  		cd ulambda
		export SIGMADEBUG="PPROC;BOOT;TEST;BENCH;IMGD"
		export SIGMAPERF="TEST_TPT;BENCH_TPT;"
  		$COMMAND
ENDSSH
	PERF_DIR=$ROOT_DIR/benchmarks/results/sigma-simple-$VERSION
	echo "collecting results to $PERF_DIR"
	rm -rf $PERF_DIR
	./collect-results.sh --parallel --perfdir $PERF_DIR
}

bench_sigmaos_sn() {
	echo "Running sigmaos bench for social network and image resizing"
	cd $SCRIPT_DIR
	./stop-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes
	./stop-sigmaos.sh --parallel
	./start-sigmaos.sh --pull $PULL --n 5 --branch social-network-benchmark
	COMMAND="
		go clean -testcache;
		go test -v sigmaos/benchmarks --no-shutdown -timeout 0 --run TestRealmBalanceSocialNetworkImgResize --mongourl $mongo_url --rootNamedIP $named_ip --imgresize_path $img_path --n_imgresize $n_resize --n_imgresize_per 1 --sn_dur $sn_dur --sn_max_rps $sn_max_rps >& test.out
	"
	echo "Run [$SSHVM]: $COMMAND"
	ssh -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM <<ENDSSH
  		# Make sure swap is off on the benchmark machines.
  		sudo swapoff -a
  		cd ulambda
		export SIGMADEBUG="PPROC;BOOT;TEST;MONGO_ERR;BENCH;IMGD"
		export SIGMAPERF="SOCIAL_NETWORK_FRONTEND_TPT;TEST_TPT;BENCH_TPT;NAMED_CPU"
  		$COMMAND
ENDSSH
	PERF_DIR=$ROOT_DIR/benchmarks/results/sigma-$VERSION
	echo "collecting results to $PERF_DIR"
	rm -rf $PERF_DIR
	./collect-results.sh --parallel --perfdir $PERF_DIR
	scp -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM:~/ulambda/test.out $PERF_DIR/bench.out
}

bench_k8s_sn() {
	echo "Running k8s bench for social network and image resizing"
	cd $SCRIPT_DIR
	aws s3 --profile me-mit rm --recursive s3://9ps3/social-network-perf/ > /dev/null
	aws s3 --profile me-mit rm --recursive s3://9ps3/img/ > /dev/null
    aws s3 --profile me-mit cp --recursive s3://9ps3/img-save/ s3://9ps3/img/ > /dev/null
	./stop-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes
	./stop-sigmaos.sh --parallel
	./stop-k8s.sh --parallel
	./start-sigmaos.sh --pull $PULL --n 5 --branch social-network-benchmark
	./start-k8s.sh --taint 5:11
	sleep 1
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/db-caches --nrunning 4
	sleep 1
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/consul --nrunning 5
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/jaeger --nrunning 6
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/user --nrunning 7
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/graph --nrunning 8
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/url --nrunning 9
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/text --nrunning 10
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/media --nrunning 11
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/post --nrunning 12
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/timeline --nrunning 13
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/home --nrunning 14
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/compose --nrunning 15
	./start-k8s-app.sh --path DeathStarBench/socialNetworkK8s/kubernetes/servers/frontend --nrunning 16

	frontend_ip=$(ssh -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM "kubectl get svc | grep front | tr -s ' ' | cut -d ' ' -f3")
	COMMAND="
		go clean -testcache;
		cp -r benchmarks/k8s/apps/thumbnail/yaml/thumbnail-heavy/ /tmp/
		sed -i 's/: XXX/: $n_resize_k8/g' /tmp/thumbnail-heavy/*
		kubectl delete -Rf /tmp/thumbnail-heavy/
		go test -v sigmaos/benchmarks --no-shutdown -timeout 0 --run TestK8sSocialNetworkImgResize --rootNamedIP $named_ip --sn_dur $sn_dur --sn_max_rps $sn_max_rps --k8saddr $frontend_ip:5000 >& test_k8s.out
	"
	echo "Run [$SSHVM]: $COMMAND"
	ssh -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM <<ENDSSH
  		# Make sure swap is off on the benchmark machines.
  		sudo swapoff -a
  		cd ulambda
		export SIGMADEBUG="PPROC;BOOT;TEST;MONGO_ERR;BENCH;IMGD"
		export SIGMAPERF="SOCIAL_NETWORK_FRONTEND_TPT;TEST_TPT;BENCH_TPT;NAMED_CPU"
  		$COMMAND
ENDSSH
	PERF_DIR_K8S=$ROOT_DIR/benchmarks/results/k8s-$VERSION
	echo "collecting results to $PERF_DIR_K8S"
	rm -rf $PERF_DIR_K8S
	./collect-results.sh --parallel --perfdir $PERF_DIR_K8S
	scp -i $SCRIPT_DIR/keys/cloudlab-sigmaos $LOGIN@$SSHVM:~/ulambda/test_k8s.out $PERF_DIR_K8S/bench.out
	aws s3 --profile me-mit rm --recursive s3://9ps3/img/ > /dev/null
}

# ========== Plot functions ==========

graph_simaos_simple() {
	PERF_DIR=$ROOT_DIR/benchmarks/results/sigma-simple-$VERSION
	cd $GRAPH_SCRIPTS_DIR
	echo "========== Graphing $PERF_DIR =========="
	python3 $GRAPH_SCRIPTS_DIR/aggregate-tpt-sn-simple.py --measurement_dir $PERF_DIR --mr_realm benchrealm1 --hotel_realm benchrealm2 --total_ncore 4 --title "Core Utilization of 2 Realms' Applications" --out $GRAPH_OUT_DIR/sigma-simple-$VERSION-result
}

graph_sigmaos_sn() {
	PERF_DIR=$ROOT_DIR/benchmarks/results/sigma-$VERSION
	cd $GRAPH_SCRIPTS_DIR
	echo "========== Graphing $PERF_DIR =========="
	python3 $GRAPH_SCRIPTS_DIR/aggregate-tpt-sn.py --measurement_dir $PERF_DIR $MR_ARG --hotel_realm benchrealm2 --total_ncore 16 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --out $GRAPH_OUT_DIR/sigma-$VERSION-result
}

graph_k8s_sn() {
	PERF_DIR_K8S=$ROOT_DIR/benchmarks/results/k8s-$VERSION
	cd $GRAPH_SCRIPTS_DIR
	echo "========== Graphing $PERF_DIR_K8S =========="
	python3 $GRAPH_SCRIPTS_DIR/aggregate-tpt-sn.py --measurement_dir $PERF_DIR_K8S $MR_ARG --hotel_realm benchrealm2 --total_ncore 16 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --out $GRAPH_OUT_DIR/k8s-$VERSION-result
}


# ========== Run benchmarks ==========
#bench_sigmaos_sn
#bench_k8s_sn
#bench_sigmaos_simple

graph_sigmaos_sn
#graph_k8s_sn
#graph_simaos_simple
