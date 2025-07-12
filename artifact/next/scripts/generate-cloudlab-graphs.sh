#!/bin/bash

VERSION=NEXT
#VERSION=NEXT_NO_MOUNT

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

for N_CACHE in 1 2 4 ; do
  # CosSim scaling
  echo "Generating eager delegated RPC init cossim graph..."
  $GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
    --input_load_label "cossim-srv" \
    --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_no_scale_cossim_nsrv_2 \
    --measurement_dir_k8s     $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_delegate_scale_cossim_add_1 \
    --out $GRAPH_OUT_DIR/cstl_delegate_nc_${N_CACHE}.pdf \
    --be_realm "" --hotel_realm benchrealm1 \
    --units "Req/sec,2-srv,Scale 1→2 srv" \
    --title "x" --total_ncore 32 --prefix "imgresize-" \
    --xmin 10000 --xmax 65000 #--legend_on_right 
  echo "Done generating eager cossim graph..."
  
  # CosSim scaling
  echo "Generating eager direct RPC init cossim graph..."
  $GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
    --input_load_label "cossim-srv" \
    --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_no_scale_cossim_nsrv_2 \
    --measurement_dir_k8s     $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_scale_cossim_add_1 \
    --out $GRAPH_OUT_DIR/cstl_nc_${N_CACHE}.pdf \
    --be_realm "" --hotel_realm benchrealm1 \
    --units "Req/sec,2-srv,Scale 1→2 srv" \
    --title "x" --total_ncore 32 --prefix "imgresize-" \
    --xmin 10000 --xmax 65000 #--legend_on_right 
  echo "Done generating eager cossim graph..."
done
