#!/usr/bin/env python3

import argparse
import os
import re
import sys
import matplotlib.pyplot as plt
import numpy as np


def parse_app_load_since_spawn(logs_out_path):
    """
    Parse logs.out and return the sinceSpawn value (in ms) for the
    Paper.Initialization.AppLoadState line.
    """
    pattern = re.compile(r'Paper\.Initialization\.AppLoadState.*sinceSpawn:(\d+(?:\.\d+)?)(ms|µs|us|s)')

    if not os.path.isfile(logs_out_path):
        print(f"Error: File not found: {logs_out_path}", file=sys.stderr)
        return None

    result = None
    try:
        with open(logs_out_path, 'r') as f:
            for line in f:
                m = pattern.search(line)
                if m:
                    value = float(m.group(1))
                    unit = m.group(2)
                    if unit in ('µs', 'us'):
                        value /= 1000.0
                    elif unit == 's':
                        value *= 1000.0
                    result = value
    except Exception as e:
        print(f"Warning: Could not read {logs_out_path}: {e}", file=sys.stderr)

    if result is None:
        print(f"Error: No Paper.Initialization.AppLoadState sinceSpawn found in {logs_out_path}", file=sys.stderr)

    return result


BLINK_BENCHMARKS = [
    ("imgrec", "Image\nRecognition"),
]


def main():
    parser = argparse.ArgumentParser(
        description="Create bar graph comparing blink proc startup times with/without co-sandbox."
    )
    for key, _ in BLINK_BENCHMARKS:
        parser.add_argument(
            f"--dir_path_{key}",
            required=True,
            help=f"Path to {key} benchmark output directory (without co-sandbox)"
        )
        parser.add_argument(
            f"--dir_path_{key}_cosandbox",
            required=True,
            help=f"Path to {key} benchmark output directory (with co-sandbox)"
        )
    parser.add_argument(
        "--offset",
        type=float,
        default=1440.0,
        help="Offset in ms to subtract from sinceSpawn values (default: 1440)"
    )
    parser.add_argument(
        "--output",
        default="blink-start-latency-cosandbox-comparison.png",
        help="Output filename for the graph (default: blink-start-latency-cosandbox-comparison.png)"
    )

    args = parser.parse_args()

    data = {}
    for key, label in BLINK_BENCHMARKS:
        plain_dir = getattr(args, f"dir_path_{key}")
        cosandbox_dir = getattr(args, f"dir_path_{key}_cosandbox")

        plain_val = parse_app_load_since_spawn(os.path.join(plain_dir, "logs.out"))
        cosandbox_val = parse_app_load_since_spawn(os.path.join(cosandbox_dir, "logs.out"))

        data[key] = {
            'label': label,
            'without_cosandbox': (plain_val - args.offset) if plain_val is not None else None,
            'with_cosandbox':    (cosandbox_val - args.offset) if cosandbox_val is not None else None,
        }

    if all(v['without_cosandbox'] is None and v['with_cosandbox'] is None for v in data.values()):
        print("Error: No data found for any blink benchmark", file=sys.stderr)
        sys.exit(1)

    keys = [k for k, _ in BLINK_BENCHMARKS]
    proc_labels     = [data[k]['label']            for k in keys]
    without_cosandbox = [data[k]['without_cosandbox'] or 0 for k in keys]
    with_cosandbox    = [data[k]['with_cosandbox']    or 0 for k in keys]

    x = np.arange(len(proc_labels))
    width = 0.35

    fig, ax = plt.subplots(figsize=(8.0, 2.4))
    bars1 = ax.bar(x - width/2, without_cosandbox, width, label='Without co-sandbox', color='steelblue')
    bars2 = ax.bar(x + width/2, with_cosandbox,    width, label='With co-sandbox',    color='coral')

    ax.set_ylabel('Start time (ms)', fontsize=12)
    ax.set_xticks([])

    ax.legend(loc='upper center', bbox_to_anchor=(0.5, -0.05), ncol=2)
    ax.grid(axis='y', alpha=0.3, linestyle='--')

    def add_value_labels(bars):
        for bar in bars:
            height = bar.get_height()
            if height > 0:
                ax.text(bar.get_x() + bar.get_width() / 2., height,
                        f'{height:.0f}ms',
                        ha='center', va='bottom', fontsize=9)

    add_value_labels(bars1)
    add_value_labels(bars2)

    y_max = max(max(without_cosandbox), max(with_cosandbox))
    ax.set_ylim(0, y_max * 1.15)

    plt.tight_layout()
    plt.savefig(args.output, dpi=300, bbox_inches='tight')
    print(f"Graph saved to {args.output}")

    print("\nSummary:")
    print("=" * 80)
    for key, label in BLINK_BENCHMARKS:
        wo = data[key]['without_cosandbox']
        cs = data[key]['with_cosandbox']
        parts = []
        if wo is not None:
            parts.append(f"Without co-sandbox: {wo:.2f}ms")
        if cs is not None:
            parts.append(f"With co-sandbox: {cs:.2f}ms")
        if wo is not None and cs is not None:
            diff = wo - cs
            pct = diff / wo * 100
            parts.append(f"CoSandbox saving: {diff:.2f}ms ({pct:.1f}%)")
        if parts:
            print(f"{label:20} | " + " | ".join(parts))
        else:
            print(f"{label:20} | Data missing")


if __name__ == "__main__":
    main()
