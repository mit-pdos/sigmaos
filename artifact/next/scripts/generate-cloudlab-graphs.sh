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

if [ "" ] ; then
# Hotel Match (slow load change)
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

# Hotel Match (fast load change)
echo "Generating hotel match (fast) graph..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py \
  --input_load_label "hotel-wwwd" \
  --measurement_dir_sigmaos $RES_OUT_DIR/hotel_match_tail_latency_fast_csdi \
  --measurement_dir_k8s     $RES_OUT_DIR/hotel_match_tail_latency_fast \
  --out $GRAPH_OUT_DIR/hotel_match_fast.pdf \
  --be_realm "" --hotel_realm benchrealm1 \
  --units "Req/sec,InitScript,No InitScript" \
  --title "x" --total_ncore 32 --prefix "val.out" \
  --client_tpt_step_size 10 --perf_step_size 10 \
  --xmin 73000 --xmax 79000 #--legend_on_right 
#  --client_tpt_step_size 10 --perf_step_size 10
echo "Done generating hotel (fast) match graph..."

echo "Generating hotel match (fast) cost graph..."
$GRAPH_SCRIPTS_DIR/deployment-cost.py \
  --input_load_label "hotel-wwwd" \
  --measurement_dir_initscripts $RES_OUT_DIR/hotel_match_tail_latency_fast_csdi \
  --measurement_dir_noinitscripts     $RES_OUT_DIR/hotel_match_tail_latency_fast \
  --out $GRAPH_OUT_DIR/hotel_match_depcost_fast.pdf \
  --xmin 74 --xmax 77 #--legend_on_right 
#  --units "Req/sec,InitScript,No InitScript" \
#  --title "x" --total_ncore 32 --prefix "imgresize-" \
#  --client_tpt_step_size 10 --perf_step_size 10 \
echo "Done generating hotel match (fast) cost graph..."
fi

echo "Generating hotel match cached hit rate graph..."
$GRAPH_SCRIPTS_DIR/match-cached-miss-rate.py \
  --measurement_dir_initscripts $RES_OUT_DIR/hotel_match_tail_latency_migrate_csdi \
  --measurement_dir_noinitscripts     $RES_OUT_DIR/hotel_match_tail_latency_migrate \
  --window_size 100000 \
  --output $GRAPH_OUT_DIR/hotel_match_migrate_cached_miss_rate.pdf #\
#  --xmin 74 --xmax 77 #--legend_on_right 
#  --units "Req/sec,InitScript,No InitScript" \
#  --title "x" --total_ncore 32 --prefix "imgresize-" \
#  --client_tpt_step_size 10 --perf_step_size 10 \
echo "Done generating hotel match cached hit rate graph..."
