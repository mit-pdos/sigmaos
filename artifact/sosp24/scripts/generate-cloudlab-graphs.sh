#!/bin/bash

VERSION=SOSP24_CAMERA_READY_FINAL

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs


## Figure 8
#echo "Generating Figure 8..."
#$GRAPH_SCRIPTS_DIR/schedd-scalability-hockey.py --measurement_dir $RES_OUT_DIR/sched_scalability --out $GRAPH_OUT_DIR/sched_scalability.pdf --prefix "23-vm-" --regex ".*E2e spawn time since spawn until main" --tpt_v_tpt > /dev/null
#echo "Done generating Figure 8..."
#
## Figure 11
#echo "Generating Figure 9..."
#$GRAPH_SCRIPTS_DIR/microservices-perf-bar-graph.py --out $GRAPH_OUT_DIR/microservices_perf.pdf --hotel_res_dir $RES_OUT_DIR/hotel_tail_latency --socialnet_res_dir $RES_OUT_DIR/socialnet_tail_latency --hotel_overlays_res_dir xxx --socialnet_overlays_res_dir xxx
#echo "Done generating Figure 9..."
##
## Figure 13
#echo "Generating Figure 13..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/lc_be_hotel_imgresize_rpc_multiplexing --out $GRAPH_OUT_DIR/lc_be_hotel_imgresize_rpc_multiplexing.pdf --be_realm benchrealm2 --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating Figure 13..."
##
## Figure 7
#echo "Generating Figure 7..."
#$GRAPH_SCRIPTS_DIR/net-microbench-talk.py --out $GRAPH_OUT_DIR/net_microbench.pdf
#echo "Done generating Figure 7..."
#
## Single-machine scalability figure
#echo "Generating single-machine scalability graph..."
#$GRAPH_SCRIPTS_DIR/schedd-scalability.py --measurement_dir $RES_OUT_DIR/single_machine_max_tpt --out $GRAPH_OUT_DIR/single_machine_max_tpt.pdf #--tpt_v_tpt > /dev/null
#echo "Done generating single-machine scalability graph..."
#
## Procq scalability
#echo "Generating procq scalability graph..."
#$GRAPH_SCRIPTS_DIR/schedd-scalability-hockey.py --measurement_dir $RES_OUT_DIR/procq_max_tpt --out $GRAPH_OUT_DIR/procq_max_tpt.pdf --prefix "23-vm-" --regex ".*Uproc Run dummy proc" --tpt_v_tpt
#echo "Done generating procq scalability graph..."

# Figure 11
echo "Generating msvc talk graph..."
$GRAPH_SCRIPTS_DIR/microservices-perf-bar-graph-talk.py --out $GRAPH_OUT_DIR/microservices_perf_talk.pdf --hotel_res_dir $RES_OUT_DIR/hotel_tail_latency --socialnet_res_dir $RES_OUT_DIR/socialnet_tail_latency --hotel_overlays_res_dir xxx --socialnet_overlays_res_dir xxx
echo "Done generating msvc talk graph..."

