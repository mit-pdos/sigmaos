#!/bin/bash

VERSION=CGROUPS

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

# Figure 13
echo "Generating cgroups graph..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/lc_be_hotel_spin_imgresize_rpc_multiplexing --out $GRAPH_OUT_DIR/lc_be_hotel_spin_imgresize_rpc_multiplexing.pdf --be_realm benchrealm2 --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
echo "Done generating cgropus graph..."
