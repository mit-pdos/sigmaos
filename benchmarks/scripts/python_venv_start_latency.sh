#!/usr/bin/env bash

set -euo pipefail

# Default values
NPROCS=1

# Parse flags
while getopts "n:" opt; do
  case $opt in
    n) NPROCS="$OPTARG" ;;
    *) echo "Usage: $0 [-n zygote_nprocs] workload1 workload2 ..." >&2; exit 1 ;;
  esac
done
shift $((OPTIND - 1))

if [ "$#" -eq 0 ]; then
  echo "Provide at least one workload" >&2
  exit 1
fi

WORKLOADS=("$@")
MODES=("private" "no_pyc" "default")

cd "$(dirname "$0")/../../"

for workload in "${WORKLOADS[@]}"; do
  echo ""
  for mode in "${MODES[@]}"; do
    output=$(
      ./stop.sh && \
      SIGMAPYMGRMODE="$mode" \
      go test -v sigmaos/benchmarks -timeout 0 \
        --start \
        --run TestPythonVenvStartLatency \
        --zygote_workload "$workload" \
        --zygote_nprocs "$NPROCS"
    )

    # Extract values
    start=$(echo "$output" | awk -F'since_spawn_mean=' '/containerStartLat:/ {
      split($2, a, " "); print a[1]
    }')

    import=$(echo "$output" | awk -F'since_spawn_mean=' '/importLatency:/ {
      split($2, a, " "); print a[1]
    }')

    echo "${workload} ${mode} start: ${start} import: ${import}"
  done
done