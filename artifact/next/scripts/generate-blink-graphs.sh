#!/bin/bash

VERSION=EUROSYS2027

ROOT_DIR=$(realpath $(dirname $0)/../../..)
RES_OUT_DIR=$ROOT_DIR/benchmarks/results/$VERSION
GRAPH_SCRIPTS_DIR=$ROOT_DIR/benchmarks/scripts/graph
GRAPH_OUT_DIR=$ROOT_DIR/benchmarks/results/graphs

echo "Generating blink start latency comparison..."
$GRAPH_SCRIPTS_DIR/blink-start-latency-cosandbox-bar-graph.py \
    --dir_path_imgrec         $RES_OUT_DIR/blink_nocosandbox \
    --dir_path_imgrec_cosandbox $RES_OUT_DIR/blink_cosandbox \
    --output $GRAPH_OUT_DIR/blink-start-latency.pdf
echo "Done generating blink start latency comparison..."
