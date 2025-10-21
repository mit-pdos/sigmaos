#!/bin/bash

VERSION=NEXT

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

#for N_CACHE in 1 2 4 ; do
#  # CosSim scaling
#  echo "Generating eager delegated RPC init cossim graph..."
#  $GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
#    --input_load_label "cossim-srv" \
#    --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_no_scale_cossim_nsrv_2 \
#    --measurement_dir_k8s     $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_delegate_scale_cossim_add_1 \
#    --out $GRAPH_OUT_DIR/cstl_delegate_nc_${N_CACHE}.pdf \
#    --be_realm "" --hotel_realm benchrealm1 \
#    --units "Req/sec,2-srv,Scale 1→2 srv" \
#    --title "x" --total_ncore 32 --prefix "imgresize-" \
#    --xmin 10000 --xmax 65000 #--legend_on_right 
#  echo "Done generating eager cossim graph..."
#  
#  # CosSim scaling
#  echo "Generating eager direct RPC init cossim graph..."
#  $GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
#    --input_load_label "cossim-srv" \
#    --measurement_dir_sigmaos $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_no_scale_cossim_nsrv_2 \
#    --measurement_dir_k8s     $RES_OUT_DIR/cos_sim_tail_latency_ncache_${N_CACHE}_eager_scale_cossim_add_1 \
#    --out $GRAPH_OUT_DIR/cstl_nc_${N_CACHE}.pdf \
#    --be_realm "" --hotel_realm benchrealm1 \
#    --units "Req/sec,2-srv,Scale 1→2 srv" \
#    --title "x" --total_ncore 32 --prefix "imgresize-" \
#    --xmin 10000 --xmax 65000 #--legend_on_right 
#  echo "Done generating eager cossim graph..."
#done
#
## Cached scaling
#echo "Generating cached scaling graphs..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
#  --input_load_label "cached-" \
#  --measurement_dir_sigmaos $RES_OUT_DIR/cached_scaler_tail_latency \
#  --measurement_dir_k8s     $RES_OUT_DIR/cached_scaler_tail_latency_delegate \
#  --out $GRAPH_OUT_DIR/cached_scale.pdf \
#  --be_realm "" --hotel_realm benchrealm1 \
#  --units "Req/sec,Direct RPC,Delegated RPC" \
#  --title "x" --total_ncore 32 --prefix "imgresize-" \
#  --xmin 40000 --xmax 45000 #--legend_on_right 
#echo "Done generating cached scaling graphs..."
#
#echo "Generating cached scaling graphs..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
#  --input_load_label "cached-" \
#  --measurement_dir_sigmaos $RES_OUT_DIR/cached_scaler_tail_latency_cossim_backend \
#  --measurement_dir_k8s     $RES_OUT_DIR/cached_scaler_tail_latency_delegate_cossim_backend \
#  --out $GRAPH_OUT_DIR/cached_scale_cs.pdf \
#  --be_realm "" --hotel_realm benchrealm1 \
#  --units "Req/sec,Direct RPC,Delegated RPC" \
#  --title "x" --total_ncore 32 --prefix "imgresize-" \
#  --xmin 45000 --xmax 55000 #--legend_on_right 
##  --xmin 45000 --xmax 50000 #--legend_on_right 
#echo "Done generating cached scaling graphs..."
#
#echo "Generating cached scaling graphs..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
#  --input_load_label "cached-" \
#  --measurement_dir_sigmaos $RES_OUT_DIR/cached_scaler_tail_latency_cpp_cossim_backend \
#  --measurement_dir_k8s     $RES_OUT_DIR/cached_scaler_tail_latency_cpp_delegate_cossim_backend \
#  --out $GRAPH_OUT_DIR/cached_scale_cpp_cs.pdf \
#  --be_realm "" --hotel_realm benchrealm1 \
#  --units "Req/sec,Direct RPC,Delegated RPC" \
#  --title "x" --total_ncore 32 --prefix "imgresize-" \
#  --xmin 45000 --xmax 55000 #--legend_on_right 
##  --xmin 45000 --xmax 50000 #--legend_on_right 
#echo "Done generating cached scaling graphs..."

# CosSim scaling
echo "Generating hotel match graph..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
  --input_load_label "hotel-wwwd" \
  --measurement_dir_sigmaos $RES_OUT_DIR/hotel_match_tail_latency_csdi \
  --measurement_dir_k8s     $RES_OUT_DIR/hotel_match_tail_latency \
  --out $GRAPH_OUT_DIR/hotel_match.pdf \
  --be_realm "" --hotel_realm benchrealm1 \
  --units "Req/sec,InitScript,No InitScript" \
  --title "x" --total_ncore 32 --prefix "imgresize-" #\
#  --xmin 10000 --xmax 65000 #--legend_on_right 
echo "Done generating hotel match graph..."
