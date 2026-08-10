#!/bin/bash

VERSION=PHD_THESIS

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

# Parse optional --sys-name and --sys-name-camel arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --sys-name)
            SYS_NAME="$2"
            shift 2
            ;;
        --sys-name-camel)
            SYS_NAME_CAMEL="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Validate: if SYS_NAME was set, SYS_NAME_CAMEL must also be set
if [[ -n "${SYS_NAME+x}" ]] && [[ -z "${SYS_NAME_CAMEL+x}" ]]; then
    echo "Error: --sys-name-camel must be set when --sys-name is set" >&2
    exit 1
fi

# Set defaults
SYS_NAME="${SYS_NAME:-co-sandbox}"
SYS_NAME_CAMEL="${SYS_NAME_CAMEL:-CoSandbox}"

# Figure 10
echo "Generating MR word count graph (new)..."
MR_RES_DIR=$RES_OUT_DIR/mr_vs_corral
MR_UX=$MR_RES_DIR/mr-wc-wiki10G-bench.json-warm
MR_S3=$MR_RES_DIR/mr-wc-wiki10G-bench-s3.json-warm
# Every configuration the MR data-path sweep produces. Any of these which
# hasn't been run yet is simply skipped by the graph script.
$GRAPH_SCRIPTS_DIR/mr_vs_corral_warm_only.py \
  --out $GRAPH_OUT_DIR/mr_vs_corral_warm_only.pdf \
  --error_bars \
  --app wc \
  --ux_dir $MR_UX \
  --s3_dir $MR_S3 \
  --s3_cosandbox_both_dir $MR_S3-cosandbox-both \
  --corral_dir $MR_RES_DIR/corral-wc-wiki10G-warm \
  --sys-name "$SYS_NAME"
#  --ux_cosandbox_both_dir $MR_UX-cosandbox-both \
#  --ux_getput_mapper_dir $MR_UX-getput-mapper \
#  --ux_getput_reducer_dir $MR_UX-getput-reducer \
#  --ux_getput_both_dir $MR_UX-getput-both \
#  --ux_cosandbox_mapper_dir $MR_UX-cosandbox-mapper \
#  --ux_cosandbox_reducer_dir $MR_UX-cosandbox-reducer \
#  --s3_getput_mapper_dir $MR_S3-getput-mapper \
#  --s3_getput_reducer_dir $MR_S3-getput-reducer \
#  --s3_getput_both_dir $MR_S3-getput-both \
#  --s3_cosandbox_mapper_dir $MR_S3-cosandbox-mapper \
#  --s3_cosandbox_reducer_dir $MR_S3-cosandbox-reducer \

echo "Done generating MR word count graph (new)..."

# The grep workload runs on its own dataset size (see corralApps in TestCorral
# and mrApps in TestMR), so it gets its own graph rather than sharing the
# word-count one's axes.
echo "Generating MR vs corral grep graph..."
GREP_UX=$MR_RES_DIR/mr-grep-wiki2G-bench.json-warm
GREP_S3=$MR_RES_DIR/mr-grep-wiki2G-granular-bench-s3.json-warm
$GRAPH_SCRIPTS_DIR/mr_vs_corral_warm_only.py \
  --out $GRAPH_OUT_DIR/mr_vs_corral_warm_only_grep.pdf \
  --app grep \
  --s3_dir $GREP_S3 \
  --s3_cosandbox_both_dir $GREP_S3-cosandbox-both \
  --corral_dir $MR_RES_DIR/corral-grep-wiki2G-warm \
  --sys-name "$SYS_NAME"
#  --ux_dir $GREP_UX \
#  --ux_cosandbox_both_dir $GREP_UX-cosandbox-both \
echo "Done generating MR vs corral grep graph..."

## Figure 12
#echo "Generating Figure 12..."
#$GRAPH_SCRIPTS_DIR/bebe-tpt.py --measurement_dir $RES_OUT_DIR/be_imgresize_rpc_multiplexing --out $GRAPH_OUT_DIR/be_imgresize_rpc_multiplexing.pdf --nrealm 4 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 40 --prefix "imgresize-"
#echo "Done generating Figure 12..."

export COSANDBOXES=""
export COSANDBOXES="_cosandboxes"

echo "Generating MR+MR multiplexing graph..."
$GRAPH_SCRIPTS_DIR/bebe-tpt.py \
  --measurement_dir $RES_OUT_DIR/be_mr_multiplexing_mem3000${COSANDBOXES}_dedicatedux8 \
  --out $GRAPH_OUT_DIR/be_mr_multiplexing.pdf --nrealm 1 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 128 --prefix "mr-" --xmax 70000
echo "Done generating MR+MR multiplexing graph..."

# Where the cluster's CPU goes while the MR job above runs: needs the run to have
# been made with CPU_MON in SIGMADEBUG, and prints the per-service breakdown it
# graphs. --include_procs adds the workload's own CPU, to show the
# infrastructure's share of the node rather than just its magnitude. fsuxd is
# split between the machines the job dedicated to serving its data and the rest,
# which is what says whether dedicating them was worth it.
echo "Generating infrastructure CPU utilization graph..."
$GRAPH_SCRIPTS_DIR/cpu-util.py --measurement_dir $RES_OUT_DIR/be_mr_multiplexing_mem3000${COSANDBOXES}_dedicatedux8 --out $GRAPH_OUT_DIR/be_mr_multiplexing_cpu_util.pdf --title "Infrastructure CPU Utilization" --xmax 70
$GRAPH_SCRIPTS_DIR/cpu-util.py --measurement_dir $RES_OUT_DIR/be_mr_multiplexing_mem3000${COSANDBOXES}_dedicatedux8 --out $GRAPH_OUT_DIR/be_mr_multiplexing_cpu_util_all.pdf --title "Cluster CPU Utilization" --include_procs --xmax 70
echo "Done generating infrastructure CPU utilization graph..."
