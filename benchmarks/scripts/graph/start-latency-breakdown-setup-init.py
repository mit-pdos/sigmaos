#!/usr/bin/env python3

import argparse
import glob
import os
import re
import sys


def find_proc_pid(dir_path, proc_name):
    """
    Search for log lines matching "Scale proc_name.*" in bench.out.* files
    and extract the proc_pid from the second matching line.
    """
    pattern = f"Scale {re.escape(proc_name)}.*"
    bench_files = glob.glob(os.path.join(dir_path, "bench.out.*"))

    if not bench_files:
        print(f"Error: No bench.out.* files found in {dir_path}", file=sys.stderr)
        sys.exit(1)

    matching_lines = []

    for bench_file in sorted(bench_files):
        try:
            with open(bench_file, 'r') as f:
                for line in f:
                    if re.search(pattern, line):
                        matching_lines.append(line.strip())
        except Exception as e:
            print(f"Warning: Could not read {bench_file}: {e}", file=sys.stderr)

    if len(matching_lines) < 2:
        print(f"Error: Found {len(matching_lines)} matching lines, need at least 2", file=sys.stderr)
        print(f"Pattern: {pattern}", file=sys.stderr)
        sys.exit(1)

    # Get the second matching line and extract the last word
    second_line = matching_lines[1]
    proc_pid = second_line.split()[-1]

    return proc_pid


def parse_timing_line(line):
    """
    Parse a log line to extract phase, operation name, sinceSpawn, and op timing.
    Expected format: [proc_pid] Setup.OperationName or Initialization.OperationName ... sinceSpawn:123ms ... op:456ms
    Returns tuple of (phase, op_name, since_spawn_ms, op_time_ms) or None if parsing fails
    """
    # Extract the phase (Setup or Initialization) and operation name
    op_match = re.search(r'\] (Setup|Initialization)\.(\S+)', line)
    if not op_match:
        return None

    phase = op_match.group(1)
    op_name = op_match.group(2)

    # Extract sinceSpawn timing
    spawn_match = re.search(r'sinceSpawn:(\d+(?:\.\d+)?)(ms|µs|us|s)', line)
    since_spawn_ms = None
    if spawn_match:
        timing_value = float(spawn_match.group(1))
        timing_unit = spawn_match.group(2)
        # Convert to milliseconds
        if timing_unit in ['µs', 'us']:
            since_spawn_ms = timing_value / 1000.0
        elif timing_unit == 's':
            since_spawn_ms = timing_value * 1000.0
        else:  # ms
            since_spawn_ms = timing_value

    # Extract op timing
    op_match_timing = re.search(r'op:(\d+(?:\.\d+)?)(ms|µs|us|s)', line)
    op_time_ms = None
    if op_match_timing:
        timing_value = float(op_match_timing.group(1))
        timing_unit = op_match_timing.group(2)
        # Convert to milliseconds
        if timing_unit in ['µs', 'us']:
            op_time_ms = timing_value / 1000.0
        elif timing_unit == 's':
            op_time_ms = timing_value * 1000.0
        else:  # ms
            op_time_ms = timing_value

    return (phase, op_name, since_spawn_ms, op_time_ms)


def find_setup_init_lines(dir_path, proc_pid):
    """
    Search all log files in dir_path/sigmaos-node-logs for lines matching
    "[proc_pid] Setup\\.*" or "[proc_pid] Initialization\\.*"
    Returns two dicts: setup_timings and init_timings, each mapping operation names
    to (sinceSpawn, op_time) tuples. If duplicates exist, keeps the last occurrence.
    """
    log_dir = os.path.join(dir_path, "sigmaos-node-logs")

    if not os.path.isdir(log_dir):
        print(f"Error: Directory {log_dir} does not exist", file=sys.stderr)
        sys.exit(1)

    log_files = glob.glob(os.path.join(log_dir, "*"))

    if not log_files:
        print(f"Error: No log files found in {log_dir}", file=sys.stderr)
        sys.exit(1)

    # Patterns to match
    setup_pattern = re.compile(rf"\[{re.escape(proc_pid)}\] Setup\..*")
    init_pattern = re.compile(rf"\[{re.escape(proc_pid)}\] Initialization\..*")

    # Use separate dicts for Setup and Initialization timings
    setup_timings = {}
    init_timings = {}

    for log_file in sorted(log_files):
        # Skip directories
        if os.path.isdir(log_file):
            continue

        try:
            with open(log_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if setup_pattern.search(line) or init_pattern.search(line):
                        parsed = parse_timing_line(line)
                        if parsed:
                            phase, op_name, since_spawn_ms, op_time_ms = parsed
                            if phase == "Setup":
                                setup_timings[op_name] = (since_spawn_ms, op_time_ms)
                            else:  # Initialization
                                init_timings[op_name] = (since_spawn_ms, op_time_ms)
        except Exception as e:
            print(f"Warning: Could not read {log_file}: {e}", file=sys.stderr)

    return setup_timings, init_timings


def main():
    parser = argparse.ArgumentParser(
        description="Extract start latency breakdown (setup/init) from SigmaOS logs"
    )
    parser.add_argument(
        "--dir_path",
        required=True,
        help="Path to directory containing benchmark output"
    )
    parser.add_argument(
        "--proc_name",
        required=True,
        help="Proc name for which to print breakdown"
    )

    args = parser.parse_args()

    # Find the proc_pid
    proc_pid = find_proc_pid(args.dir_path, args.proc_name)
    print(f"Found proc_pid: {proc_pid}")
    print()

    # Find setup/initialization timings
    setup_timings, init_timings = find_setup_init_lines(args.dir_path, proc_pid)

    if not setup_timings and not init_timings:
        print("No setup/initialization timings found")
        return

    # Print Setup section
    if setup_timings:
        print("=" * 80)
        print("SETUP LATENCIES")
        print("=" * 80)
        print(f"{'Operation':<40} {'op (ms)':<20} {'sinceSpawn (ms)':<20}")
        print("-" * 80)

        for op_name, (since_spawn_ms, op_time_ms) in sorted(setup_timings.items()):
            spawn_str = f"{since_spawn_ms:.3f}" if since_spawn_ms is not None else "N/A"
            op_str = f"{op_time_ms:.3f}" if op_time_ms is not None else "N/A"
            print(f"{op_name:<40} {op_str:<20} {spawn_str:<20}")
        print()

    # Print Initialization section
    if init_timings:
        print("=" * 80)
        print("INITIALIZATION LATENCIES")
        print("=" * 80)
        print(f"{'Operation':<40} {'op (ms)':<20} {'sinceSpawn (ms)':<20}")
        print("-" * 80)

        for op_name, (since_spawn_ms, op_time_ms) in sorted(init_timings.items()):
            spawn_str = f"{since_spawn_ms:.3f}" if since_spawn_ms is not None else "N/A"
            op_str = f"{op_time_ms:.3f}" if op_time_ms is not None else "N/A"
            print(f"{op_name:<40} {op_str:<20} {spawn_str:<20}")


if __name__ == "__main__":
    main()
