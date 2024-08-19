package remote

import (
	sp "sigmaos/sigmap"
)

// Realm IDs for benchmarking
const (
	REALM1 sp.Trealm = "benchrealm1"
	REALM2 sp.Trealm = "benchrealm2"
)

// Script directories, relative to project root directory
const (
	AWS_DIR_REL          string = "aws"
	CLOUDLAB_DIR_REL            = "cloudlab"
	GRAPH_SCRIPT_DIR_REL        = "benchmarks/scripts/graph"
)

// Output directories, relative to project root directory
const (
	OUTPUT_PARENT_DIR_REL string = "benchmarks/results"
	GRAPH_OUTPUT_DIR_REL         = OUTPUT_PARENT_DIR_REL + "/graphs"
)

// Log files
const (
	CLUSTER_INIT_LOG  string = "/tmp/init.out"
	BENCH_NODE_LOGS   string = "sigmaos-node-logs"
	NODE_LOG_DIR      string = "/tmp/" + BENCH_NODE_LOGS
	BENCH_CONFIG_FILE string = "bench-config.txt"
)
