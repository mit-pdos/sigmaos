#!/bin/bash

#VERSION=SOSP24_CAMERA_READY_FINAL
#VERSION=SOSP24_TALK_STAGING
#VERSION=SOSP24_TALK_FINAL

VERSION=SOSP24_HOTEL_CLOUDLAB

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

## Figure 6
#echo "Generating Figure 6..."
#$GRAPH_SCRIPTS_DIR/start-latency.py --out $GRAPH_OUT_DIR/start_latency.pdf --cold_res_dir $RES_OUT_DIR/cold_start --warm_res_dir $RES_OUT_DIR/cold_start
#echo "Done generating Figure 6..."
#
# Figure 10
#echo "Generating Figure 10..."
#$GRAPH_SCRIPTS_DIR/mr_vs_corral.py --measurement_dir $RES_OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/mr_vs_corral.pdf --datasize=10G
#echo "Done generating Figure 10..."

## Figure 10 (new)
#echo "Generating Figure 10 (new)..."
#$GRAPH_SCRIPTS_DIR/mr_vs_corral_warm_only.py --measurement_dir $RES_OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/mr_vs_corral_warm_only.pdf --datasize=10G
#echo "Done generating Figure 10 (new)..."
##
## Figure 12
#echo "Generating Figure 12..."
#$GRAPH_SCRIPTS_DIR/bebe-tpt.py --measurement_dir $RES_OUT_DIR/be_imgresize_rpc_multiplexing --out $GRAPH_OUT_DIR/be_imgresize_rpc_multiplexing.pdf --nrealm 4 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 96 --prefix "imgresize-"
#echo "Done generating Figure 12..."

## MR fine-grained
#echo "Generating MR fine-grained..."
#$GRAPH_SCRIPTS_DIR/mr_vs_corral_warm_only.py --measurement_dir $RES_OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/mr_vs_corral_fine_grained.pdf --datasize=2G --granular --app "grep" --noux
#echo "Done generating MR fine-grained..."

##
## Hotel elasticity
#echo "Generating hotel elasticity..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_2.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity..."
#
##
## Hotel elasticity
#echo "Generating hotel elasticity..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_0 --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_0.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity..."

##
## Hotel elasticity
#echo "Generating hotel no elasticity, many caches..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_no_scale_cache_ncache_3 --out $GRAPH_OUT_DIR/hotel_tail_latency_no_scale_cache_ncache_3.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" #--xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel no elasticity, many caches..."

#echo "Generating hotel no elasticity, few caches..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_no_scale_cache_ncache_1 --out $GRAPH_OUT_DIR/hotel_tail_latency_no_scale_cache_ncache_1.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel no elasticity, few caches..."
#
#echo "Generating hotel elasticity with 200ms delay..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_200ms --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_200ms.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity with 200ms delay..."
#
#echo "Generating hotel elasticity with 500ms delay..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_500ms --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_500ms.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity with 500ms delay..."
#
#echo "Generating hotel elasticity with 1s delay..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_1s --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_1s.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity with 1s delay..."
#
#echo "Generating hotel elasticity with 2s delay..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_2s --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_cache_add_2_extra_scaling_delay_2s.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "Aggregate Throughput Balancing 2 Realms' Applications" --total_ncore 32 --prefix "imgresize-" --xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity with 2s delay..."

## Hotel elasticity
#echo "Generating hotel no elasticity, many geos..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_no_scale_geo_ngeo_3 --out $GRAPH_OUT_DIR/hotel_tail_latency_no_scale_geo_ngeo_3.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel no elasticity, many geos..."

## Hotel elasticity
#echo "Generating hotel elasticity, add 2 geos..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_geo_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_geo_add_2.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity, add 2 geos..."
#
## Hotel elasticity
#echo "Generating hotel elasticity, add 2 geos 1s delay..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_scale_geo_add_2_extra_scaling_delay_1s --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_geo_add_2_extra_scaling_delay_1s.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 100000 --xmax 225000 #--legend_on_right 
#echo "Done generating hotel elasticity, add 2 geos 1s delay..."

# XXX we care about these below

# Hotel elasticity side-by-side
echo "Generating hotel georeq elasticity, side-by-side..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py --measurement_dir_sigmaos $RES_OUT_DIR/hotel_tail_latency_georeq_scale_geo_add_2 --measurement_dir_k8s $RES_OUT_DIR/hotel_tail_latency_georeq_k8s_scale_geo_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_georeq_scale_geo_side_by_side.pdf --be_realm "" --hotel_realm benchrealm1 --units "Req/sec,SigmaOS Lat (ms),K8s Lat (ms)" --title "x" --total_ncore 32 --prefix "imgresize-" --xmin 24000 --xmax 35000 #--legend_on_right 
echo "Done generating hotel georeq elasticity, side-by-side..."

# Hotel elasticity
echo "Generating hotel georeq elasticity, no scale 1 geo..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_georeq_no_scale_geo_ngeo_1 --out $GRAPH_OUT_DIR/hotel_tail_latency_georeq_no_scale_geo_ngeo_1.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 340000 --xmax 440000 #--legend_on_right 
echo "Done generating georeq hotel elasticity, no scale 1 geo..."

# Hotel elasticity
echo "Generating hotel georeq elasticity, no scale 3 geo..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_georeq_no_scale_geo_ngeo_3 --out $GRAPH_OUT_DIR/hotel_tail_latency_georeq_no_scale_geo_ngeo_3.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 340000 --xmax 440000 #--legend_on_right 
echo "Done generating georeq hotel elasticity, no scale 3 geo..."

# Hotel elasticity
echo "Generating hotel georeq elasticity, scale up to 2 geo..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_georeq_scale_geo_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_georeq_scale_geo_add_2.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 340000 --xmax 440000 #--legend_on_right 
echo "Done generating georeq hotel elasticity, scale up to 2 geo..."

## Hotel elasticity
#echo "Generating k8s hotel elasticity, scale up to 2 geo..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_k8s_scale_geo_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_k8s_scale_geo_add_2.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" --xmin 340000 --xmax 440000 #--legend_on_right 
#echo "Done generating k8s hotel elasticity, scale up to 2 geo..."

# Hotel elasticity side-by-side
echo "Generating hotel elasticity, side-by-side..."
$GRAPH_SCRIPTS_DIR/aggregate-tpt-talk.py --measurement_dir_sigmaos $RES_OUT_DIR/hotel_tail_latency_scale_geo_add_2 --measurement_dir_k8s $RES_OUT_DIR/hotel_tail_latency_k8s_scale_geo_add_2 --out $GRAPH_OUT_DIR/hotel_tail_latency_scale_geo_side_by_side.pdf --be_realm "" --hotel_realm benchrealm1 --units "Req/sec,SigmaOS Lat (ms),K8s Lat (ms)" --title "x" --total_ncore 32 --prefix "imgresize-" --xmin 20000 --xmax 35000 #--legend_on_right 
echo "Done generating hotel elasticity, side-by-side..."

## Hotel elasticity
#echo "Generating k8s hotel no elasticity, 1 small geo..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_1 --out $GRAPH_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_1_small.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" --xmin 370000 --xmax 420000
#echo "Done generating k8s hotel no elasticity, 1 small geo..."
#
## Hotel elasticity
#echo "Generating k8s hotel no elasticity, 3 small geo..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_3 --out $GRAPH_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_3_small.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" --xmin 360000 --xmax 410000 #--legend_on_right 
#echo "Done generating k8s hotel no elasticity, 3 small geo..."

# Hotel elasticity
#echo "Generating k8s hotel no elasticity, one small geo..."
#$GRAPH_SCRIPTS_DIR/aggregate-tpt.py --measurement_dir $RES_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_1 --out $GRAPH_OUT_DIR/hotel_tail_latency_k8s_no_scale_geo_ngeo_1_big.pdf --be_realm "" --hotel_realm benchrealm1 --units "Latency (ms),Req/sec,MB/sec" --title "x" --total_ncore 32 --prefix "imgresize-" #--xmin 1000000 --xmax 1200000 #--legend_on_right 
#echo "Done generating k8s hotel no elasticity, one small geo..."
