#!/usr/bin/env python3

import argparse
import glob
import os
import re
import sys
import matplotlib.pyplot as plt
import numpy as np


# ─── shared utilities (identical to start-latency-cosandbox-bar-graph.py) ────

def find_proc_pid(dir_path, proc_name, start=True):
    """
    Search for log lines matching "Scale proc_name.*" in bench.out.* files
    and extract the proc_pid from the matching line.
    For start mode: uses first matching line
    """
    action = "Scale"
    pattern = f"{action} {re.escape(proc_name)}.*"
    bench_files = glob.glob(os.path.join(dir_path, "bench.out.*"))

    if not bench_files:
        print(f"Error: No bench.out.* files found in {dir_path}", file=sys.stderr)
        return None

    matching_lines = []

    for bench_file in sorted(bench_files):
        try:
            with open(bench_file, 'r') as f:
                for line in f:
                    if re.search(pattern, line):
                        matching_lines.append(line.strip())
        except Exception as e:
            print(f"Warning: Could not read {bench_file}: {e}", file=sys.stderr)

    if start:
        if len(matching_lines) < 1:
            print(f"Error: Found {len(matching_lines)} matching lines for {proc_name} in {dir_path}, need at least 1", file=sys.stderr)
            print(f"Pattern: {pattern}", file=sys.stderr)
            return None
        target_line = matching_lines[0]
    else:
        if len(matching_lines) < 2:
            print(f"Error: Found {len(matching_lines)} matching lines for {proc_name} in {dir_path}, need at least 2", file=sys.stderr)
            print(f"Pattern: {pattern}", file=sys.stderr)
            return None
        target_line = matching_lines[1]

    # Extract the last word as proc_pid
    proc_pid = target_line.split()[-1]

    return proc_pid


def parse_timing_line(line):
    """
    Parse a log line to extract phase, operation name, sinceSpawn, and op timing.
    Expected format: [proc_pid] [Paper.]Setup.OperationName or [Paper.]Initialization.OperationName ... sinceSpawn:123ms ... op:456ms
    Returns tuple of (phase, op_name, since_spawn_ms, op_time_ms) or None if parsing fails
    """
    op_match = re.search(r'\] (?:Paper\.)?(Setup|Initialization)\.(\S+)', line)
    if not op_match:
        return None

    phase = op_match.group(1)
    op_name = op_match.group(2)

    spawn_match = re.search(r'sinceSpawn:(\d+(?:\.\d+)?)(ms|µs|us|s)', line)
    since_spawn_ms = None
    if spawn_match:
        timing_value = float(spawn_match.group(1))
        timing_unit = spawn_match.group(2)
        if timing_unit in ['µs', 'us']:
            since_spawn_ms = timing_value / 1000.0
        elif timing_unit == 's':
            since_spawn_ms = timing_value * 1000.0
        else:
            since_spawn_ms = timing_value

    op_match_timing = re.search(r'op:(\d+(?:\.\d+)?)(ms|µs|us|s)', line)
    op_time_ms = None
    if op_match_timing:
        timing_value = float(op_match_timing.group(1))
        timing_unit = op_match_timing.group(2)
        if timing_unit in ['µs', 'us']:
            op_time_ms = timing_value / 1000.0
        elif timing_unit == 's':
            op_time_ms = timing_value * 1000.0
        else:
            op_time_ms = timing_value

    return (phase, op_name, since_spawn_ms, op_time_ms)


def find_setup_init_lines(dir_path, proc_pid):
    """
    Search all log files in dir_path/sigmaos-node-logs for lines matching
    "[proc_pid] [Paper.]Setup\\.*" or "[proc_pid] [Paper.]Initialization\\.*"
    Returns two dicts: setup_timings and init_timings, each mapping operation names
    to (sinceSpawn, op_time) tuples. If duplicates exist, keeps the last occurrence.
    """
    log_dir = os.path.join(dir_path, "sigmaos-node-logs")

    if not os.path.isdir(log_dir):
        print(f"Error: Directory {log_dir} does not exist", file=sys.stderr)
        return {}, {}

    log_files = glob.glob(os.path.join(log_dir, "*"))

    if not log_files:
        print(f"Error: No log files found in {log_dir}", file=sys.stderr)
        return {}, {}

    escaped_pid = re.escape(proc_pid)
    setup_pattern = re.compile(rf"\[{escaped_pid}\] (?:Paper\.)?Setup\..*")
    init_pattern = re.compile(rf"\[{escaped_pid}\] (?:Paper\.)?Initialization\..*")

    setup_timings = {}
    init_timings = {}

    for log_file in sorted(log_files):
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
                            else:
                                init_timings[op_name] = (since_spawn_ms, op_time_ms)
        except Exception as e:
            print(f"Warning: Could not read {log_file}: {e}", file=sys.stderr)

    return setup_timings, init_timings


def get_setup_and_init_times(dir_path, proc_name):
    """
    Returns (setup_span_ms, init_start_ms, init_span_ms):
      setup_span  = end of last setup op - start of first setup op (bar bottom always 0)
      init_start  = start of first init op (sinceSpawn - opTime, clamped to 0)
      init_span   = end of last init op - init_start
    Returns (None, None, None) on failure.
    """
    proc_pid = find_proc_pid(dir_path, proc_name, start=True)
    if proc_pid is None:
        return None, None, None

    setup_timings, init_timings = find_setup_init_lines(dir_path, proc_pid)

    def phase_bounds(timings):
        end_ms = start_ms = None
        for _, (since_spawn_ms, op_time_ms) in timings.items():
            if since_spawn_ms is None:
                continue
            if end_ms is None or since_spawn_ms > end_ms:
                end_ms = since_spawn_ms
            s = max(0.0, since_spawn_ms - op_time_ms) if op_time_ms is not None else 0.0
            if start_ms is None or s < start_ms:
                start_ms = s
        return start_ms, end_ms

    _, setup_end         = phase_bounds(setup_timings)
    init_start, init_end = phase_bounds(init_timings)

    if setup_end is None and init_end is None:
        return None, None, None

    setup_span = setup_end if setup_end is not None else 0.0
    init_start = init_start or 0.0
    init_span  = (init_end - init_start) if init_end is not None else 0.0
    return setup_span, init_start, init_span


def get_last_init_time(dir_path, proc_name):
    """
    Extract the time since spawn for the last initialization step for a given proc.
    Returns the max sinceSpawn value from all initialization steps, or None if not found.
    """
    proc_pid = find_proc_pid(dir_path, proc_name, start=True)
    if proc_pid is None:
        return None

    _, init_timings = find_setup_init_lines(dir_path, proc_pid)

    if not init_timings:
        print(f"Warning: No initialization timings found for {proc_name} in {dir_path}", file=sys.stderr)
        return None

    max_since_spawn = None
    for op_name, (since_spawn_ms, op_time_ms) in init_timings.items():
        if since_spawn_ms is not None:
            if max_since_spawn is None or since_spawn_ms > max_since_spawn:
                max_since_spawn = since_spawn_ms

    return max_since_spawn


# ─── SeBS-specific ────────────────────────────────────────────────────────────

# All SeBS benchmarks spawn "sebs-runner.py" as the proc.
SEBS_PROC_NAME = "sebs-runner.py"

# Ordered list of (arg_key, display_label) pairs.
SEBS_BENCHMARKS = [
    ("thumbnailer",       "Thumbnailer*"),
    ("video_processing",  "Video\nProcessing*"),
    ("image_recognition", "Image\nRecognition*"),
    ("dna_visualisation", "DNA\nVisualisation*"),
    ("sleep",             "Sleep"),
    ("dynamic_html",      "Dynamic\nHTML"),
    ("graph_pagerank",    "Graph\nPagerank"),
    ("graph_mst",         "Graph\nMST"),
    ("graph_bfs",         "Graph\nBFS"),
]

# Non-SeBS benchmarks that can be mixed into the same graph.
# Each entry: (arg_key, display_label, proc_name).
# These do not support --show-uncompressed.
EXTRA_BENCHMARKS = [
    ("etcd",      "Etcd",      "etcd-shim"),
    ("memcached", "Memcached", "memcached-shim"),
]


def main():
    parser = argparse.ArgumentParser(
        description="Create bar graph comparing SeBS proc startup times with/without co-sandbox. "
                    "Pass --show-uncompressed to include a third bar for uncompressed bundles."
    )
    parser.add_argument(
        "--show-uncompressed",
        action="store_true",
        default=False,
        help="Include uncompressed-bundle bars alongside compressed and co-sandbox bars"
    )
    for key, _ in SEBS_BENCHMARKS:
        parser.add_argument(
            f"--dir_path_{key}",
            default=None,
            help=f"Path to {key} benchmark output directory (plain, compressed)"
        )
        parser.add_argument(
            f"--dir_path_{key}_uncompressed",
            default=None,
            help=f"Path to {key} benchmark output directory (plain, uncompressed); "
                 f"required when --show-uncompressed is set"
        )
        parser.add_argument(
            f"--dir_path_{key}_cosandbox",
            default=None,
            help=f"Path to {key} with co-sandbox benchmark output directory"
        )
    for key, _, _ in EXTRA_BENCHMARKS:
        parser.add_argument(
            f"--dir_path_{key}",
            default=None,
            help=f"Path to {key} benchmark output directory"
        )
        parser.add_argument(
            f"--dir_path_{key}_cosandbox",
            default=None,
            help=f"Path to {key} with co-sandbox benchmark output directory"
        )
    parser.add_argument(
        "--show_breakdown",
        action="store_true",
        help="Overlay setup and initialization time sub-bars inside each end-to-end bar"
    )
    parser.add_argument(
        "--output",
        default="sebs-start-latency-cosandbox-comparison.png",
        help="Output filename for the graph (default: sebs-start-latency-cosandbox-comparison.png)"
    )
    parser.add_argument(
        "--sys-name",
        default="co-sandbox",
        help="Label to use in place of 'co-sandbox' in legend entries (default: co-sandbox)"
    )
    parser.add_argument(
        "--figsize",
        nargs=2,
        type=float,
        metavar=("WIDTH", "HEIGHT"),
        default=None,
        help="Figure size in inches, e.g. --figsize 12.0 3.0 (defaults: 9.0x2.4 for uncompressed, 16.0x2.4 otherwise)"
    )

    args = parser.parse_args()

    # Filter to benchmarks where both plain and cosandbox dirs are supplied.
    # active_benchmarks is a list of (key, label, proc_name) triples.
    active_benchmarks = [
        (key, label, SEBS_PROC_NAME) for key, label in SEBS_BENCHMARKS
        if getattr(args, f"dir_path_{key}") is not None
        and getattr(args, f"dir_path_{key}_cosandbox") is not None
    ] + [
        (key, label, proc_name) for key, label, proc_name in EXTRA_BENCHMARKS
        if getattr(args, f"dir_path_{key}") is not None
        and getattr(args, f"dir_path_{key}_cosandbox") is not None
    ]

    if not active_benchmarks:
        parser.error("No benchmarks specified. Provide at least one --dir_path_<benchmark> and --dir_path_<benchmark>_cosandbox pair.")

    # SeBS-only keys (extras don't support --show-uncompressed)
    active_sebs_keys = {key for key, _, _ in active_benchmarks if key in dict([(k, l) for k, l in SEBS_BENCHMARKS])}
    if args.show_uncompressed:
        for key in active_sebs_keys:
            if getattr(args, f"dir_path_{key}_uncompressed") is None:
                parser.error(f"--dir_path_{key}_uncompressed is required when --show-uncompressed is set")

    # Collect timing data for each benchmark.
    data = {}
    for key, label, proc_name in active_benchmarks:
        plain_dir = getattr(args, f"dir_path_{key}")
        cosandbox_dir = getattr(args, f"dir_path_{key}_cosandbox")
        entry = {
            'label': label,
            'compressed':     get_last_init_time(plain_dir, proc_name),
            'with_cosandbox': get_last_init_time(cosandbox_dir, proc_name),
        }
        if args.show_uncompressed and key in active_sebs_keys:
            uncompressed_dir = getattr(args, f"dir_path_{key}_uncompressed")
            entry['uncompressed'] = get_last_init_time(uncompressed_dir, proc_name)
        data[key] = entry

    if all(v['compressed'] is None and v['with_cosandbox'] is None for v in data.values()):
        print("Error: No data found for any benchmark", file=sys.stderr)
        sys.exit(1)

    breakdown = None
    if args.show_breakdown and not args.show_uncompressed:
        breakdown = {}
        for key, _, proc_name in active_benchmarks:
            plain_dir     = getattr(args, f"dir_path_{key}")
            cosandbox_dir = getattr(args, f"dir_path_{key}_cosandbox")
            breakdown[key] = {
                'compressed':     get_setup_and_init_times(plain_dir,     proc_name),
                'with_cosandbox': get_setup_and_init_times(cosandbox_dir, proc_name),
            }

    keys = [k for k, _, _ in active_benchmarks]
    proc_labels   = [data[k]['label']         for k in keys]
    compressed    = [data[k]['compressed']     or 0 for k in keys]
    with_cosandbox = [data[k]['with_cosandbox'] or 0 for k in keys]

    x = np.arange(len(proc_labels)) * 0.3

    from matplotlib.patches import Patch

    if args.show_uncompressed:
        uncompressed = [data[k]['uncompressed'] or 0 for k in keys]
        width = 0.08
        fig, ax = plt.subplots(figsize=args.figsize or (9.0, 2.4))
        bars1 = ax.bar(x - width, compressed,     width, label='Compressed',      color='steelblue')
        bars2 = ax.bar(x,         uncompressed,   width, label='Uncompressed',    color='seagreen')
        bars3 = ax.bar(x + width, with_cosandbox, width, label=f'With {args.sys_name}', color='coral')
        y_max = max(max(compressed), max(uncompressed), max(with_cosandbox))

        def add_value_labels(bars):
            for bar in bars:
                height = bar.get_height()
                if height > 0:
                    ax.text(bar.get_x() + bar.get_width()/2., height,
                            f'{height:.0f}ms', ha='center', va='bottom', fontsize=9)
        for bars in [bars1, bars2, bars3]:
            add_value_labels(bars)

        legend_handles = [
            Patch(facecolor='steelblue', label='Compressed'),
            Patch(facecolor='seagreen',  label='Uncompressed'),
            Patch(facecolor='coral',     label=f'With {args.sys_name}'),
        ]
        ncol = 3
    else:
        width = 0.12
        fig, ax = plt.subplots(figsize=args.figsize or (16.0, 2.4))

        if breakdown:
            sub_w  = (width - 0.01) / 2
            offset = sub_w / 2 + 0.005

            setup_co  = [breakdown[k]['compressed'][0]     or 0 for k in keys]
            i_start_co = [breakdown[k]['compressed'][1]    or 0 for k in keys]
            init_co   = [breakdown[k]['compressed'][2]     or 0 for k in keys]
            setup_cs  = [breakdown[k]['with_cosandbox'][0] or 0 for k in keys]
            i_start_cs = [breakdown[k]['with_cosandbox'][1] or 0 for k in keys]
            init_cs   = [breakdown[k]['with_cosandbox'][2] or 0 for k in keys]

            ax.bar(x - width/2, compressed,     width, fill=False, edgecolor='steelblue', linewidth=1.5)
            ax.bar(x + width/2, with_cosandbox, width, fill=False, edgecolor='coral',     linewidth=1.5)
            ax.bar(x - width/2 - offset, setup_co, sub_w, color='steelblue')
            ax.bar(x + width/2 - offset, setup_cs, sub_w, color='coral')
            ax.bar(x - width/2 + offset, init_co, sub_w, bottom=i_start_co, color='steelblue', hatch='///', edgecolor='white', linewidth=0)
            ax.bar(x + width/2 + offset, init_cs, sub_w, bottom=i_start_cs, color='coral',     hatch='///', edgecolor='white', linewidth=0)

            legend_handles = [
                Patch(facecolor='steelblue', label=f'Without {args.sys_name}'),
                Patch(facecolor='coral',     label=f'With {args.sys_name}'),
                Patch(facecolor='lightgrey', edgecolor='grey', label='Setup'),
                Patch(facecolor='lightgrey', edgecolor='grey', hatch='///', label='Initialization'),
            ]
            ncol = 2
        else:
            ax.bar(x - width/2, compressed,     width, color='steelblue')
            ax.bar(x + width/2, with_cosandbox, width, color='coral')
            legend_handles = [
                Patch(facecolor='steelblue', label=f'Without {args.sys_name}'),
                Patch(facecolor='coral',     label=f'With {args.sys_name}'),
            ]
            ncol = 2

        for xpos, h in zip(x - width/2, compressed):
            if h > 0:
                ax.text(xpos, h, f'{h:.0f}ms', ha='center', va='bottom', fontsize=9)
        for xpos, h in zip(x + width/2, with_cosandbox):
            if h > 0:
                ax.text(xpos, h, f'{h:.0f}ms', ha='center', va='bottom', fontsize=9)

        y_max = max(max(compressed), max(with_cosandbox))

    ax.set_ylabel('Start time (ms)', fontsize=12)
    ax.set_xlim(x[0] - width - 0.02, x[-1] + width + 0.02)
    ax.set_xticks(x)
    ax.set_xticklabels(proc_labels)
    ax.grid(axis='y', alpha=0.3, linestyle='--')
    ax.legend(handles=legend_handles, loc='lower center', bbox_to_anchor=(0.5, 1.0),
              ncol=ncol, fontsize=10, borderpad=0.3, handletextpad=0.5,
              columnspacing=1.0, frameon=True)
    ax.set_ylim(0, y_max * 1.15)

    plt.tight_layout()
    plt.savefig(args.output, dpi=300, bbox_inches='tight')
    print(f"Graph saved to {args.output}")

    print("\nSummary:")
    print("=" * 100)
    for key, label, _ in active_benchmarks:
        c  = data[key]['compressed']
        u  = data[key].get('uncompressed')
        cs = data[key]['with_cosandbox']
        parts = []
        if c is not None:
            parts.append(f"Compressed: {c:.2f}ms")
        if u is not None:
            parts.append(f"Uncompressed: {u:.2f}ms")
            if c is not None:
                diff = c - u
                pct = diff / c * 100
                parts.append(f"Uncomp saving: {diff:.2f}ms ({pct:.1f}%)")
        if cs is not None:
            parts.append(f"Co-sandbox: {cs:.2f}ms")
            if c is not None:
                diff = c - cs
                pct = diff / c * 100
                parts.append(f"CoSandbox saving: {diff:.2f}ms ({pct:.1f}%)")
        if parts:
            print(f"{label:20} | " + " | ".join(parts))
        else:
            print(f"{label:20} | Data missing")


if __name__ == "__main__":
    main()
