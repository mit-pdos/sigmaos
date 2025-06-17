#!/bin/bash

VERSION=NEXT

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

# CosSim scaling
echo "Generating eager cossim graph..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
  --input_load_label "cossim-srv" \
  --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_eager_no_scale_cossim_nsrv_2 \
  --measurement_dir_k8s $RES_OUT_DIR/cos_sim_tail_latency_eager_scale_cossim_add_1 \
  --out $GRAPH_OUT_DIR/cossim_tail_latency_eager_scale_vs_noscale.pdf \
  --be_realm "" --hotel_realm benchrealm1 \
  --units "Req/sec,2-srv,Scale 1->2 srv" \
  --title "x" --total_ncore 32 --prefix "imgresize-" \
  --xmin 10000 --xmax 65000 #--legend_on_right 
echo "Done generating eager cossim graph..."

# CosSim scaling
echo "Generating lazy cossim graph..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
  --input_load_label "cossim-srv" \
  --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_no_scale_cossim_nsrv_2 \
  --measurement_dir_k8s $RES_OUT_DIR/cos_sim_tail_latency_scale_cossim_add_1 \
  --out $GRAPH_OUT_DIR/cossim_tail_latency_lazy_scale_vs_noscale.pdf \
  --be_realm "" --hotel_realm benchrealm1 \
  --units "Req/sec,2-srv,Scale 1->2 srv" \
  --title "x" --total_ncore 32 --prefix "imgresize-" \
  --xmin 10000 --xmax 65000 #--legend_on_right 
echo "Done generating lazy cossim graph..."
