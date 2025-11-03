#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import re
import sys

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

def read_csv_data(fpath):
    """Read CSV data with unix timestamps and values."""
    with open(fpath, "r") as f:
        x = f.read()
    lines = [l.strip().split(",") for l in x.split("\n") if len(l.strip()) > 0]
    # Parse timestamp (remove 'us' suffix) and value
    data = [(float(l[0].replace("us", "")), float(l[1])) for l in lines]
    return data

def find_file(directory, pattern):
    """Find a file matching the given regex pattern in the directory."""
    regex = re.compile(pattern)
    for f in os.listdir(directory):
        if regex.search(f):
            return os.path.join(directory, f)
    return None

def bucket_data(data, bucket_size_us=5000):
    """Bucket data into time buckets and sum the values.

    Args:
        data: List of (timestamp_us, value) tuples
        bucket_size_us: Bucket size in microseconds (default: 5000us = 5ms)

    Returns:
        Dictionary mapping bucket_start_timestamp to sum of values in that bucket
    """
    buckets = {}
    for ts, val in data:
        # Calculate which bucket this timestamp falls into
        bucket_start = (int(ts) // bucket_size_us) * bucket_size_us
        buckets[bucket_start] = buckets.get(bucket_start, 0) + val
    return buckets

def calculate_miss_rate(hits, misses, bucket_size_us=5000):
    """Calculate miss rate from hits and misses data, bucketed into time intervals.

    Args:
        hits: List of (timestamp, hit_count) tuples
        misses: List of (timestamp, miss_count) tuples
        bucket_size_us: Bucket size in microseconds (default: 5000us = 5ms)

    Returns:
        List of (bucket_timestamp, miss_rate) tuples where miss_rate is in percentage.
    """
    # Bucket the data
    hit_buckets = bucket_data(hits, bucket_size_us)
    miss_buckets = bucket_data(misses, bucket_size_us)

    # Get all unique bucket timestamps
    all_buckets = sorted(set(list(hit_buckets.keys()) + list(miss_buckets.keys())))

    miss_rates = []
    for bucket_ts in all_buckets:
        hit_val = hit_buckets.get(bucket_ts, 0)
        miss_val = miss_buckets.get(bucket_ts, 0)
        total = hit_val + miss_val

        if total > 0:
            miss_rate = (miss_val / total) * 100.0
        else:
            miss_rate = 0.0

        miss_rates.append((bucket_ts, miss_rate))

    return miss_rates

def normalize_timestamps(data, start_time=None):
    """Normalize timestamps to start from 0 (or relative to start_time).

    If start_time is not provided, finds the first non-zero value and uses that timestamp.
    """
    if len(data) == 0:
        return []

    if start_time is None:
        # Find the first non-zero value
        for ts, val in data:
            if val > 0:
                start_time = ts
                break
        # If no non-zero values found, use the first timestamp
        if start_time is None:
            start_time = data[0][0]

    # Convert from microseconds to seconds
    normalized = [(ts / 1e6 - start_time / 1e6, val) for ts, val in data]
    return normalized

def main():
    parser = argparse.ArgumentParser(description='Graph cache miss rate over time')
    parser.add_argument('--measurement_dir_initscripts', type=str, required=True,
                        help='Directory containing data with init scripts')
    parser.add_argument('--measurement_dir_noinitscripts', type=str, required=True,
                        help='Directory containing data without init scripts')
    parser.add_argument('--output', type=str, default='match-cached-miss-rate.pdf',
                        help='Output file path (default: match-cached-miss-rate.pdf)')
    parser.add_argument('--window_size', type=int, default=5000,
                        help='Time bucket size in microseconds (default: 5000us = 5ms)')

    args = parser.parse_args()

    # File patterns for hit and miss data
    hit_pattern = r'hotel-matchd-.+-hit-tpt\.out'
    miss_pattern = r'hotel-matchd-.+-miss-tpt\.out'

    # Read data for initscripts configuration
    print("Reading initscripts data...")
    init_hit_file = find_file(args.measurement_dir_initscripts, hit_pattern)
    init_miss_file = find_file(args.measurement_dir_initscripts, miss_pattern)

    if not init_hit_file or not init_miss_file:
        print(f"Error: Could not find hit/miss files in {args.measurement_dir_initscripts}")
        print(f"Hit file: {init_hit_file}")
        print(f"Miss file: {init_miss_file}")
        sys.exit(1)

    init_hits = read_csv_data(init_hit_file)
    init_misses = read_csv_data(init_miss_file)
    init_miss_rate = calculate_miss_rate(init_hits, init_misses, args.window_size)

    # Read data for no_initscripts configuration
    print("Reading no_initscripts data...")
    no_init_hit_file = find_file(args.measurement_dir_noinitscripts, hit_pattern)
    no_init_miss_file = find_file(args.measurement_dir_noinitscripts, miss_pattern)

    if not no_init_hit_file or not no_init_miss_file:
        print(f"Error: Could not find hit/miss files in {args.measurement_dir_noinitscripts}")
        print(f"Hit file: {no_init_hit_file}")
        print(f"Miss file: {no_init_miss_file}")
        sys.exit(1)

    no_init_hits = read_csv_data(no_init_hit_file)
    no_init_misses = read_csv_data(no_init_miss_file)
    no_init_miss_rate = calculate_miss_rate(no_init_hits, no_init_misses, args.window_size)

    # Normalize timestamps
    init_miss_rate_norm = normalize_timestamps(init_miss_rate)
    no_init_miss_rate_norm = normalize_timestamps(no_init_miss_rate)

    # Create the plot with two vertically-aligned subplots
    fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(6.4, 4.8), sharex=True)

    # Plot init scripts data in top subplot
    if len(init_miss_rate_norm) > 0:
        times_init, rates_init = zip(*init_miss_rate_norm)
        ax1.plot(times_init, rates_init, label='With Init Scripts', linewidth=2, marker='o', markersize=3, color='tab:blue')

    ax1.grid(True, alpha=0.3)
    ax1.set_xlim(left=0)
    ax1.set_ylim(0, 100)

    # Plot no init scripts data in bottom subplot
    if len(no_init_miss_rate_norm) > 0:
        times_no_init, rates_no_init = zip(*no_init_miss_rate_norm)
        ax2.plot(times_no_init, rates_no_init, label='Without Init Scripts', linewidth=2, marker='s', markersize=3, color='tab:orange')

    ax2.set_xlabel('Time (seconds)', fontsize=14)
    ax2.grid(True, alpha=0.3)
    ax2.set_xlim(left=0)
    ax2.set_ylim(0, 100)

    # Create a single legend with both lines, with bottom anchored to top of topmost subplot
    handles1, labels1 = ax1.get_legend_handles_labels()
    handles2, labels2 = ax2.get_legend_handles_labels()
    fig.legend(handles1 + handles2, labels1 + labels2, loc='lower center',
               bbox_to_anchor=(0.5, 1.0), ncol=2, fontsize=12, frameon=True)

    # Adjust layout to make room for shared y-axis label
    plt.tight_layout()
    plt.subplots_adjust(left=0.15)

    # Add a shared y-axis label
    fig.text(0.06, 0.5, 'Cache Miss Rate (%)', va='center', rotation='vertical', fontsize=14)

    # Save the plot
    plt.savefig(args.output, dpi=300, bbox_inches='tight')

if __name__ == "__main__":
    main()
