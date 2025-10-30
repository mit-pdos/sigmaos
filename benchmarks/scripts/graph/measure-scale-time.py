#!/usr/bin/env python3

import argparse
import glob
import re
from datetime import datetime

def parse_timestamp(line):
    """Extract timestamp from log line.
    Expected format: HH:MM:SS.dddddd as first word
    """
    # Timestamp is always the first word in the log line
    parts = line.split()
    if not parts:
        return None

    timestamp_str = parts[0]
    # Parse HH:MM::SS.ddddd format (note the double colon)
    match = re.match(r'(\d{2}):(\d{2}):(\d{2})\.(\d+)', timestamp_str)
    if match:
        hours = int(match.group(1))
        minutes = int(match.group(2))
        seconds = int(match.group(3))
        microseconds = int(match.group(4))

        # Convert to total seconds with fractional part
        total_seconds = hours * 3600 + minutes * 60 + seconds + microseconds / 1_000_000
        return total_seconds

    return None

def analyze_scaling_times(file_path):
    """Analyze the time between scale up and scale down events."""
    scale_up_events = []
    scale_down_events = []

    with open(file_path, 'r') as f:
        for line in f:
            if 'Manual scale: Scale up cossim srvs by' in line:
                timestamp = parse_timestamp(line)
                if timestamp:
                    # Extract the number of servers being scaled up
                    match = re.search(r'Scale up cossim srvs by (\d+)', line)
                    num_srvs = int(match.group(1)) if match else 0
                    scale_up_events.append((timestamp, num_srvs, line.strip()))

            elif 'Manual scale: Scale down cossim srvs by' in line:
                timestamp = parse_timestamp(line)
                if timestamp:
                    # Extract the number of servers being scaled down
                    match = re.search(r'Scale down cossim srvs by (\d+)', line)
                    num_srvs = int(match.group(1)) if match else 0
                    scale_down_events.append((timestamp, num_srvs, line.strip()))

    return scale_up_events, scale_down_events

def calculate_time_differences(scale_up_events, scale_down_events):
    """Calculate time differences between consecutive scale up and scale down events."""
    intervals = []

    # Pair each scale up with the next scale down
    scale_down_idx = 0
    for up_time, up_num, up_line in scale_up_events:
        # Find the next scale down event after this scale up
        while scale_down_idx < len(scale_down_events):
            down_time, down_num, down_line = scale_down_events[scale_down_idx]

            if up_time is not None and down_time is not None:
                if down_time > up_time:
                    time_diff = down_time - up_time
                    intervals.append({
                        'scale_up': up_line,
                        'scale_down': down_line,
                        'time_diff_seconds': time_diff,
                        'scale_up_num': up_num,
                        'scale_down_num': down_num
                    })
                    scale_down_idx += 1
                    break

            scale_down_idx += 1

    return intervals

def main():
    parser = argparse.ArgumentParser(description='Measure time between scale up and scale down events')
    parser.add_argument('--dir_path', required=True, help='Directory containing bench.out.* files')
    args = parser.parse_args()

    # Find all bench.out.* files in the directory
    pattern = f"{args.dir_path}/bench.out.*"
    files = glob.glob(pattern)

    if not files:
        print(f"No bench.out.* files found in {args.dir_path}")
        return

    print(f"Found {len(files)} bench.out.* file(s)")
    print()

    all_intervals = []

    for file_path in sorted(files):
        print(f"Analyzing: {file_path}")
        scale_up_events, scale_down_events = analyze_scaling_times(file_path)

        print(f"  Found {len(scale_up_events)} scale up events")
        print(f"  Found {len(scale_down_events)} scale down events")

        intervals = calculate_time_differences(scale_up_events, scale_down_events)
        all_intervals.extend(intervals)

        if intervals:
            print(f"  Found {len(intervals)} scale up -> scale down intervals")
            for i, interval in enumerate(intervals, 1):
                print(f"    Interval {i}: {interval['time_diff_seconds']:.3f} seconds "
                      f"(up by {interval['scale_up_num']}, down by {interval['scale_down_num']})")
        print()

    if all_intervals:
        print("=" * 80)
        print("SUMMARY")
        print("=" * 80)
        print(f"Total intervals measured: {len(all_intervals)}")

        times = [interval['time_diff_seconds'] for interval in all_intervals]
        avg_time = sum(times) / len(times)
        min_time = min(times)
        max_time = max(times)

        print(f"Average time: {avg_time:.3f} seconds")
        print(f"Min time:     {min_time:.3f} seconds")
        print(f"Max time:     {max_time:.3f} seconds")
        print()

        print("Detailed intervals:")
        for i, interval in enumerate(all_intervals, 1):
            print(f"{i}. {interval['time_diff_seconds']:.3f}s "
                  f"(up by {interval['scale_up_num']}, down by {interval['scale_down_num']})")
    else:
        print("No scale up -> scale down intervals found")

if __name__ == '__main__':
    main()
