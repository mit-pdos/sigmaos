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

## Figure 10
#echo "Generating MR graph (new)..."
#$GRAPH_SCRIPTS_DIR/mr_vs_corral_warm_only.py --measurement_dir $RES_OUT_DIR/mr_vs_corral/ --out $GRAPH_OUT_DIR/mr_vs_corral_warm_only.pdf --datasize=10G
#echo "Done generating MR graph (new)..."

# Figure 12
echo "Generating Figure 12..."
$GRAPH_SCRIPTS_DIR/bebe-tpt.py --measurement_dir $RES_OUT_DIR/be_imgresize_rpc_multiplexing --out $GRAPH_OUT_DIR/be_imgresize_rpc_multiplexing.pdf --nrealm 4 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 40 --prefix "imgresize-"
echo "Done generating Figure 12..."

echo "Generating MR+MR multiplexing graph..."
$GRAPH_SCRIPTS_DIR/bebe-tpt.py --measurement_dir $RES_OUT_DIR/be_mr_multiplexing --out $GRAPH_OUT_DIR/be_mr_multiplexing.pdf --nrealm 4 --units "MB/sec" --title "Aggregate Throughput Balancing 4 Realms' BE Applications" --total_ncore 40 --prefix "mr-"
echo "Done generating MR+MR multiplexing graph..."


