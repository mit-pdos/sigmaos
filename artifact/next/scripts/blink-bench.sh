#!/usr/bin/env bash
set -euo pipefail

VERSION="EUROSYS2027"
SR03="sr03.csail.mit.edu"
SR04="sr04.csail.mit.edu"
SR04_IP="18.26.4.102"
SSH_KEY="$HOME/.ssh/aws_dev"
SSH="ssh -i $SSH_KEY"

COPY_SNAPSHOTS=false
for arg in "$@"; do
    case "$arg" in
        --copy-snapshots) COPY_SNAPSHOTS=true ;;
    esac
done

if [ "$COPY_SNAPSHOTS" = true ]; then
    echo "Copying snapshots to sr03..."
    $SSH "$SR03" "cd ~/blink-ae && ./scripts/scp_imgrec_snapshots.sh"
fi

run_bench() {
    local test_name="$1"
    local out_dir="$2"

    if [ -d "$out_dir" ]; then
        echo "Skipping $test_name: $out_dir already exists"
        return
    fi

    echo "Running $test_name..."

    # sr03: pull repo and stop NFS client
    $SSH "$SR03" "cd ~/blink-ae && git pull && ./scripts/stop_nfs.sh"

    echo "Start NFS server..."
    # sr04: pull repo, stop then start NFS server
    $SSH "$SR04" "cd ~/blink-ae && git pull && ./scripts/stop_nfs.sh && ./scripts/start_nfs_srv.sh || true"

    echo "Run benchmark..."
    # sr03: run benchmark
    $SSH "$SR03" "cd ~/sigmaos && ./stop.sh --parallel ; go clean -testcache; go test -v sigmaos/blink --run $test_name --start --blink --no-shutdown --nfs $SR04_IP 2>&1 | tee /tmp/bench.out ; ./logs.sh > /tmp/logs.out 2>&1"

    echo "SCP results..."
    mkdir -p "$out_dir"
    scp -i "$SSH_KEY" "$SR03:/tmp/bench.out" "$out_dir/"
    scp -i "$SSH_KEY" "$SR03:/tmp/logs.out" "$out_dir/"

    echo "Done running $test_name..."
}

RESULTS_BASE="./benchmarks/results/${VERSION}"

run_bench "CoSandbox" "${RESULTS_BASE}/blink_cosandbox"
run_bench "Shmem"     "${RESULTS_BASE}/blink_nocosandbox"
