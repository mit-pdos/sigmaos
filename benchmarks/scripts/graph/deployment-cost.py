#!/usr/bin/env python3

import argparse
import glob
import matplotlib.pyplot as plt
import os


def parse_csv_file(file_path, reference_start_time=None):
    """Parse CSV file with timestamp and value columns.

    Args:
        file_path: Path to the CSV file
        reference_start_time: Optional reference start time in microseconds.
                              If None, uses the first timestamp in the file.

    Returns:
        tuple: (times_seconds, values, start_time_us) where times_seconds is a list of floats
               representing seconds from the reference start, values is a list of floats,
               and start_time_us is the first timestamp in microseconds
    """
    times = []
    values = []

    with open(file_path, 'r') as f:
        for line in f:
            line = line.strip()
            if not line:
                continue

            parts = line.split(',')
            if len(parts) != 2:
                continue

            # Parse timestamp (remove 'us' suffix and convert to microseconds)
            time_str = parts[0].strip()
            if time_str.endswith('us'):
                time_str = time_str[:-2]
            time_us = int(time_str)

            # Parse value
            value = float(parts[1].strip())

            times.append(time_us)
            values.append(value)

    if not times:
        return [], [], None

    # Get the start time from this file
    file_start_time = times[0]

    # Use reference start time if provided, otherwise use this file's start time
    start_time = reference_start_time if reference_start_time is not None else file_start_time

    # Convert times to seconds relative to the start time
    times_seconds = [(t - start_time) / 1_000_000.0 for t in times]

    return times_seconds, values, file_start_time


def find_file_with_prefix(directory, prefix):
    """Find a file in the directory that starts with the given prefix.

    Returns:
        str: Path to the first matching file, or None if not found
    """
    pattern = os.path.join(directory, f"{prefix}*")
    files = glob.glob(pattern)

    if not files:
        return None

    # Return the first matching file
    return files[0]


def filter_data_by_time_range(times, values, xmin=None, xmax=None):
    """Filter data to only include points within the specified time range.

    Args:
        times: List of time values
        values: List of corresponding values
        xmin: Minimum time value (inclusive), or None for no lower bound
        xmax: Maximum time value (inclusive), or None for no upper bound

    Returns:
        tuple: (filtered_times, filtered_values)
    """
    if not times or not values:
        return times, values

    filtered_times = []
    filtered_values = []

    for t, v in zip(times, values):
        if xmin is not None and t < xmin:
            continue
        if xmax is not None and t > xmax:
            continue
        filtered_times.append(t)
        filtered_values.append(v)

    return filtered_times, filtered_values


def calculate_area_under_curve(times, values):
    """Calculate the area under the curve between the first max value and last min non-zero value.

    Args:
        times: List of time values in seconds
        values: List of corresponding values

    Returns:
        tuple: (area, start_time, end_time) where area is the integrated value,
               start_time is when we start integrating, end_time is when we stop
    """
    if not times or not values or len(times) != len(values):
        return 0.0, None, None

    # Find the maximum value
    max_value = max(values)

    # Find the minimum non-zero value
    non_zero_values = [v for v in values if v > 0]
    if not non_zero_values:
        return 0.0, None, None
    min_nonzero_value = min(non_zero_values)

    # Find the first time the value jumps to max
    start_idx = None
    for i in range(len(values)):
        if values[i] == max_value:
            start_idx = i
            break

    # Find the last time the value drops to min non-zero
    end_idx = None
    for i in range(len(values) - 1, -1, -1):
        if values[i] == min_nonzero_value:
            end_idx = i
            break

    # If we couldn't find the bounds, return 0
    if start_idx is None or end_idx is None or start_idx >= end_idx:
        return 0.0, None, None

    # Calculate area using trapezoidal rule
    area = 0.0
    for i in range(start_idx, end_idx):
        dt = times[i + 1] - times[i]
        avg_value = (values[i] + values[i + 1]) / 2.0
        area += avg_value * dt

    return area, times[start_idx], times[end_idx]


def aggregate_by_window(times, values, window_size=0.01):
    """Aggregate values by summing them within fixed time windows and scale to per-second rate.

    Args:
        times: List of time values in seconds
        values: List of corresponding values
        window_size: Size of each window in seconds (default: 0.01 = 10ms)

    Returns:
        tuple: (window_times, aggregated_values) where window_times are the
               center points of each window, and aggregated_values are the sums
               scaled to show per-second rate
    """
    if not times or not values:
        return [], []

    # Find the time range
    min_time = min(times)
    max_time = max(times)

    # Create windows
    window_times = []
    aggregated_values = []

    # Calculate scaling factor to convert window sum to per-second rate
    # window_size is in seconds, so to get per-second rate: (1.0 / window_size)
    scale_factor = 1.0 / window_size

    current_window_start = min_time
    while current_window_start < max_time:
        window_end = current_window_start + window_size
        window_center = current_window_start + window_size / 2.0

        # Sum all values that fall within this window
        window_sum = 0.0
        for t, v in zip(times, values):
            if current_window_start <= t < window_end:
                window_sum += v

        # Only include windows with non-zero values to smooth the graph
        if window_sum > 0:
            # Scale the sum to show per-second rate
            scaled_value = window_sum * scale_factor

            window_times.append(window_center)
            aggregated_values.append(scaled_value)

        current_window_start = window_end

    return window_times, aggregated_values


def main():
    parser = argparse.ArgumentParser(
        description='Graph deployment cost over time from measurement files'
    )
    parser.add_argument('--input_load_label', required=True,
                        help='Prefix of the file to search for in measurement directories')
    parser.add_argument('--measurement_dir_initscripts', required=True,
                        help='Directory containing measurements with init scripts')
    parser.add_argument('--measurement_dir_noinitscripts', required=True,
                        help='Directory containing measurements without init scripts')
    parser.add_argument('--output', required=True,
                        help='Output path for the generated graph')
    parser.add_argument('--xmin', type=float, default=None,
                        help='Minimum x-axis value (seconds) to display')
    parser.add_argument('--xmax', type=float, default=None,
                        help='Maximum x-axis value (seconds) to display')

    args = parser.parse_args()

    # Find and parse file from initscripts directory
    initscripts_file = find_file_with_prefix(
        args.measurement_dir_initscripts, args.input_load_label
    )
    if not initscripts_file:
        print(f"Error: No file with prefix '{args.input_load_label}' found in "
              f"{args.measurement_dir_initscripts}")
        return 1

    times_init, values_init, init_start_time = parse_csv_file(initscripts_file)

    # Find and parse file from noinitscripts directory
    noinitscripts_file = find_file_with_prefix(
        args.measurement_dir_noinitscripts, args.input_load_label
    )
    if not noinitscripts_file:
        print(f"Error: No file with prefix '{args.input_load_label}' found in "
              f"{args.measurement_dir_noinitscripts}")
        return 1

    times_noinit, values_noinit, noinit_start_time = parse_csv_file(noinitscripts_file)

    # Use initscripts start time as the reference
    # Both datasets will be relative to their own start (time 0), so they align at the beginning
    reference_start_time = init_start_time

    # Re-parse initscripts with its own start time (so it starts at 0)
    times_init, values_init, _ = parse_csv_file(initscripts_file, init_start_time)
    # Parse noinitscripts with its own start time (so it also starts at 0)
    times_noinit, values_noinit, _ = parse_csv_file(noinitscripts_file, noinit_start_time)

    # Filter data by time range if specified
    times_init, values_init = filter_data_by_time_range(
        times_init, values_init, args.xmin, args.xmax
    )
    times_noinit, values_noinit = filter_data_by_time_range(
        times_noinit, values_noinit, args.xmin, args.xmax
    )

    # Aggregate values by 10ms windows (summing values within each window)
    times_init, values_init = aggregate_by_window(times_init, values_init, window_size=0.01)
    times_noinit, values_noinit = aggregate_by_window(times_noinit, values_noinit, window_size=0.01)

    # Find and parse the MCPU data files (with "-val.out" suffix)
    # Look for any file ending with -val.out in the directories
    initscripts_val_pattern = os.path.join(args.measurement_dir_initscripts, "*-val.out")
    initscripts_val_files = glob.glob(initscripts_val_pattern)
    if initscripts_val_files:
        initscripts_val_file = initscripts_val_files[0]
        # Use initscripts start time so val data aligns with load data
        times_init_mcpu, values_init_mcpu, _ = parse_csv_file(initscripts_val_file, init_start_time)
        # Filter by time range
        times_init_mcpu, values_init_mcpu = filter_data_by_time_range(
            times_init_mcpu, values_init_mcpu, args.xmin, args.xmax
        )
    else:
        print(f"No -val.out file found in {args.measurement_dir_initscripts}")
        times_init_mcpu, values_init_mcpu = [], []

    noinitscripts_val_pattern = os.path.join(args.measurement_dir_noinitscripts, "*-val.out")
    noinitscripts_val_files = glob.glob(noinitscripts_val_pattern)
    if noinitscripts_val_files:
        noinitscripts_val_file = noinitscripts_val_files[0]
        # Use noinitscripts start time so val data aligns with load data
        times_noinit_mcpu, values_noinit_mcpu, _ = parse_csv_file(noinitscripts_val_file, noinit_start_time)
        # Filter by time range
        times_noinit_mcpu, values_noinit_mcpu = filter_data_by_time_range(
            times_noinit_mcpu, values_noinit_mcpu, args.xmin, args.xmax
        )
    else:
        print(f"No -val.out file found in {args.measurement_dir_noinitscripts}")
        times_noinit_mcpu, values_noinit_mcpu = [], []

    # Create the figure with three subplots
    fig, (ax1, ax2, ax3) = plt.subplots(3, 1, figsize=(6.4, 4.8), sharex=True)

    # First subplot: Client Load (RPS)
    line1, = ax1.plot(times_init, values_init, label='Client Load', linewidth=1.5)
    # ax1.plot(times_noinit, values_noinit, label='Without Init Scripts', linewidth=1.5)
    ax1.set_ylabel('Client Load (RPS)', fontsize=12)
    ax1.grid(True, alpha=0.3)
    ax1.set_ylim(bottom=0)

    # Second subplot: MCPU Reserved (With Init Scripts)
    line2, = ax2.plot(times_init_mcpu, values_init_mcpu, label='Initscripts', linewidth=1.5, color='orange')
    ax2.set_ylabel('mCPU', fontsize=12)
    ax2.grid(True, alpha=0.3)
    ax2.set_ylim(bottom=0)

    # Third subplot: MCPU Reserved (Without Init Scripts)
    line3, = ax3.plot(times_noinit_mcpu, values_noinit_mcpu, label='No Initscripts', linewidth=1.5, color='green')
    ax3.set_xlabel('Time (seconds)', fontsize=12)
    ax3.set_ylabel('mCPU', fontsize=12)
    ax3.grid(True, alpha=0.3)
    ax3.set_ylim(bottom=0)

    # Create a single horizontal legend above all subplots
    fig.legend(handles=[line1, line2, line3],
               labels=['Client Load', 'Initscripts', 'No Initscripts'],
               loc='lower center',
               ncol=3,
               fontsize=10,
               bbox_to_anchor=(0.5, 1.0))

    # Calculate area under the curve for MCPU data
    print("\n=== Area Under Curve (MCPU Reserved) ===")

    if times_init_mcpu and values_init_mcpu:
        area_init, start_init, end_init = calculate_area_under_curve(times_init_mcpu, values_init_mcpu)
        print(f"With Init Scripts:")
        if start_init is not None and end_init is not None:
            print(f"  Area: {area_init:.2f} MCPU·seconds")
            print(f"  Integration window: {start_init:.2f}s to {end_init:.2f}s (duration: {end_init - start_init:.2f}s)")
            print(f"  Max value: {max(values_init_mcpu):.2f} MCPU")
            print(f"  Min non-zero value: {min([v for v in values_init_mcpu if v > 0]):.2f} MCPU")
        else:
            print(f"  Could not calculate area (no valid range found)")
    else:
        print(f"With Init Scripts: No MCPU data available")

    if times_noinit_mcpu and values_noinit_mcpu:
        area_noinit, start_noinit, end_noinit = calculate_area_under_curve(times_noinit_mcpu, values_noinit_mcpu)
        print(f"\nWithout Init Scripts:")
        if start_noinit is not None and end_noinit is not None:
            print(f"  Area: {area_noinit:.2f} MCPU·seconds")
            print(f"  Integration window: {start_noinit:.2f}s to {end_noinit:.2f}s (duration: {end_noinit - start_noinit:.2f}s)")
            print(f"  Max value: {max(values_noinit_mcpu):.2f} MCPU")
            print(f"  Min non-zero value: {min([v for v in values_noinit_mcpu if v > 0]):.2f} MCPU")
        else:
            print(f"  Could not calculate area (no valid range found)")
    else:
        print(f"Without Init Scripts: No MCPU data available")

    # Compare the two
    if times_init_mcpu and values_init_mcpu and times_noinit_mcpu and values_noinit_mcpu:
        area_init, _, _ = calculate_area_under_curve(times_init_mcpu, values_init_mcpu)
        area_noinit, _, _ = calculate_area_under_curve(times_noinit_mcpu, values_noinit_mcpu)
        if area_init > 0 and area_noinit > 0:
            diff = area_init - area_noinit
            percent_diff = (diff / area_noinit) * 100
            print(f"\nComparison:")
            print(f"  Difference: {diff:.2f} MCPU·seconds ({percent_diff:+.1f}%)")

    # Save the plot
    plt.tight_layout()
    # Adjust layout to make room for the legend above the top plot
    plt.savefig(args.output, dpi=300, bbox_inches='tight')
    print(f"\nGraph saved to: {args.output}")

    return 0


if __name__ == '__main__':
    exit(main())
