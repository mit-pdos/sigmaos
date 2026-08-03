#!/usr/bin/env python3

import argparse
import glob
import os
import re
from datetime import datetime
import matplotlib.pyplot as plt
import matplotlib.dates as mdates

def parse_timestamp(ts_str):
    """Parse Go timestamp format: HH:MM:SS.microseconds"""
    try:
        # Go format: HH:MM:SS.microseconds
        # Since we only have time (no date), we'll use a dummy date
        # The actual date doesn't matter as we'll convert to relative time
        time_obj = datetime.strptime(ts_str, "%H:%M:%S.%f")
        # Use today's date as dummy, only time matters for relative calculations
        return time_obj.replace(year=2000, month=1, day=1)
    except ValueError:
        # Try without microseconds
        time_obj = datetime.strptime(ts_str, "%H:%M:%S")
        return time_obj.replace(year=2000, month=1, day=1)

def parse_cpu_util_file(filepath, verbose=False):
    """Parse a bench.out file and extract CPU utilization data."""
    timestamps = []
    cpu_utils = []

    with open(filepath, 'r') as f:
        for line in f:
            if "Cores utilized:" in line:
                parts = line.strip().split()
                if len(parts) >= 2:
                    # First word is the timestamp (HH:MM:SS.microseconds). The
                    # field after "utilized:" is the realm's cores; a second,
                    # comma-separated field (newer runs) is what every cgroup on
                    # those nodes used, which is not what this graph plots.
                    timestamp_str = parts[0]

                    try:
                        i = parts.index("utilized:")
                        cpu_util = float(parts[i + 1].rstrip(","))
                        timestamp = parse_timestamp(timestamp_str)
                        timestamps.append(timestamp)
                        cpu_utils.append(cpu_util)
                    except (ValueError, IndexError) as e:
                        print(f"Warning: Could not parse line: {line.strip()}")
                        if verbose:
                            print(f"  Error: {e}")
                        continue

    return timestamps, cpu_utils

def find_bench_file(directory, verbose=False):
    """Find the bench.out.* file in the given directory."""
    pattern = os.path.join(directory, "bench.out.*")
    files = glob.glob(pattern)

    if not files:
        raise FileNotFoundError(f"No bench.out.* file found in {directory}")

    if len(files) > 1:
        print(f"Warning: Multiple bench.out files found in {directory}, using first one: {files[0]}")

    return files[0]

def align_timestamps(timestamps, start_time):
    """Convert absolute timestamps to relative time in seconds from start_time."""
    return [(ts - start_time).total_seconds() for ts in timestamps]

def main():
    parser = argparse.ArgumentParser(description='Graph CPU utilization for image processing benchmarks')
    parser.add_argument('--cosandbox_dir', required=True, help='Directory containing bench.out.* for cosandbox run')
    parser.add_argument('--no_cosandbox_dir', required=True, help='Directory containing bench.out.* for no-cosandbox run')
    parser.add_argument('--output', default='imgprocess-cpu-util.png', help='Output filename for the graph')
    parser.add_argument('--verbose', '-v', action='store_true', help='Enable verbose output')

    args = parser.parse_args()

    # Find and parse both files
    if args.verbose:
        print(f"Parsing cosandbox file from {args.cosandbox_dir}")
    cosandbox_file = find_bench_file(args.cosandbox_dir, args.verbose)
    init_timestamps, init_cpu_utils = parse_cpu_util_file(cosandbox_file, args.verbose)
    if args.verbose:
        print(f"Found {len(init_timestamps)} CPU utilization measurements")

    if args.verbose:
        print(f"Parsing no-cosandbox file from {args.no_cosandbox_dir}")
    no_init_file = find_bench_file(args.no_cosandbox_dir, args.verbose)
    no_init_timestamps, no_init_cpu_utils = parse_cpu_util_file(no_init_file, args.verbose)
    if args.verbose:
        print(f"Found {len(no_init_timestamps)} CPU utilization measurements")

    if not init_timestamps or not no_init_timestamps:
        print("Error: No data found in one or both files")
        return

    # Align timestamps to start at 0
    init_start = init_timestamps[0]
    no_init_start = no_init_timestamps[0]

    init_times = align_timestamps(init_timestamps, init_start)
    no_init_times = align_timestamps(no_init_timestamps, no_init_start)

    # Create the plot
    fig, ax = plt.subplots(figsize=(6.4, 3.2))

    line1, = ax.plot(init_times, init_cpu_utils, label='With Init Script', marker='o', markersize=3, linewidth=1.5)
    line2, = ax.plot(no_init_times, no_init_cpu_utils, label='Without Init Script', marker='s', markersize=3, linewidth=1.5)

    ax.set_xlabel('Time (seconds)', fontsize=12)
    ax.set_ylabel('CPU Utilization (cores)', fontsize=12)
    ax.grid(True, alpha=0.3)

    # Set axis limits to start at 0
    ax.set_xlim(left=0)
    ax.set_ylim(bottom=0)

    # Create legend above the plot
    fig.legend(handles=[line1, line2],
               labels=['With Init Script', 'Without Init Script'],
               loc='lower center',
               ncol=2,
               fontsize=10,
               bbox_to_anchor=(0.5, 1.0))

    # Format the plot
    plt.tight_layout()

    # Save the plot
    plt.savefig(args.output, dpi=300, bbox_inches='tight')
    if args.verbose:
        print(f"Graph saved to {args.output}")

    # Print summary statistics
    if args.verbose:
        print("\nSummary Statistics:")
        print(f"With Init Script:")
        print(f"  Average CPU Utilization: {sum(init_cpu_utils)/len(init_cpu_utils):.2f} cores")
        print(f"  Max CPU Utilization: {max(init_cpu_utils):.2f} cores")
        print(f"  Duration: {init_times[-1]:.2f} seconds")

        print(f"Without Init Script:")
        print(f"  Average CPU Utilization: {sum(no_init_cpu_utils)/len(no_init_cpu_utils):.2f} cores")
        print(f"  Max CPU Utilization: {max(no_init_cpu_utils):.2f} cores")
        print(f"  Duration: {no_init_times[-1]:.2f} seconds")

if __name__ == '__main__':
    main()
