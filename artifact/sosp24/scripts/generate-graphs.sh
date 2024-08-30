#!/bin/bash

VERSION=SOSP24AE

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

# Figure 6
echo "Generating Figure 6..."
$GRAPH_SCRIPTS_DIR/start-latency.py --out $GRAPH_OUT_DIR/start_latency.pdf --cold_res_dir $RES_OUT_DIR/cold_start --warm_res_dir $RES_OUT_DIR/cold_start
echo "Done generating Figure 6..."

# Figure 8
echo "Generating Figure 8..."
$GRAPH_SCRIPTS_DIR/schedd-scalability-hockey.py --measurement_dir $RES_OUT_DIR/sched_scalability --out $GRAPH_OUT_DIR/sched_scalability.pdf --prefix "23-vm-" --tpt_v_tpt > /dev/null
echo "Done generating Figure 8..."

# Figure 10
echo "Generating Figure 10..."
$GRAPH_SCRIPTS_DIR/mr_vs_corral.py --measurement_dir $RES_OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/mr_vs_corral.pdf --datasize=2G
echo "Done generating Figure 10..."

# Figure 11
echo "Generating Figure 11..."
$GRAPH_SCRIPTS_DIR/microservices-perf-bar-graph.py --out $GRAPH_OUT_DIR/microservices_perf.pdf --hotel_res_dir $RES_OUT_DIR/hotel_tail_latency --socialnet_res_dir $RES_OUT_DIR/socialnet_tail_latency
echo "Done generating Figure 11..."

# Figure 12
echo "Generating Figure 12..."
$GRAPH_SCRIPTS_DIR/bebe-tpt.py --measurement_dir $RES_OUT_DIR/be_imgresize_multiplexing --out $GRAPH_OUT_DIR/be_imgresize_multiplexing.pdf --nrealm 4 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 32 --prefix "imgresize-"
echo "Done generating Figure 12..."

# Figure 13
echo "Generating Figure 13..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/lc_be_hotel_imgresize_multiplexing --out $GRAPH_OUT_DIR/lc_be_hotel_imgresize_multiplexing.pdf --be_realm benchrealm2 --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" #--legend_on_right 
echo "Done generating Figure 13..."
