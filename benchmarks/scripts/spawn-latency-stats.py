#!/usr/bin/env python3
"""Print statistics for each SPAWN_LATENCY log type emitted by a given proc program.

SigmaOS procs log spawn-latency measurements via perf.LogSpawnLatency, which
produce lines of the form:

    15:04:05.123456 mr-m-wc-abc123 SPAWN_LAT [mr-m-wc-abc123] Mapper.initOutput op:1.234ms sinceSpawn:5.678ms

The text between "] " and " op:" is the log type (e.g. "Mapper.initOutput"),
"op:" is the elapsed time of the operation, and "sinceSpawn:" is the elapsed
time since the proc was spawned. A single program (e.g. the MR mapper, whose
pids look like "mr-m-<app>-<rand>") emits many such lines across all of its
instances.

Given a proc --program, this script groups every matching log line by its log
type and prints the average, median, p90, and p95 of both the "op" and
"sinceSpawn" durations.

Logs are read from files passed as positional arguments, from a benchmark
output directory via --dir (which is scanned recursively), or from stdin if
neither is given. For example:

    SIGMADEBUG=SPAWN_LAT go test ... 2>&1 | spawn-latency-stats.py --program mr-m-wc
    spawn-latency-stats.py --program mr-m-wc --dir /path/to/benchmark-output
"""

import argparse
import glob
import os
import re
import sys


# Matches "[pid] <logtype> op:<dur> sinceSpawn:<dur>". The log type is captured
# non-greedily so it stops at the first " op:".
LINE_RE = re.compile(
    r"\[(?P<pid>[^\]]+)\]\s+(?P<logtype>.*?)\s+op:(?P<op>\S+)\s+sinceSpawn:(?P<spawn>\S+)"
)

# Matches one (value, unit) component of a Go duration string, e.g. the "1m",
# "2.5s" in "1m2.5s". Longer units ("ms", "ns", "µs", "us") come before the
# single-char units so alternation prefers them.
DUR_COMPONENT_RE = re.compile(r"(\d+(?:\.\d+)?)(ns|µs|us|ms|s|m|h)")

# Multiplier to convert each unit to milliseconds.
UNIT_TO_MS = {
    "ns": 1e-6,
    "µs": 1e-3,
    "us": 1e-3,
    "ms": 1.0,
    "s": 1000.0,
    "m": 60_000.0,
    "h": 3_600_000.0,
}


def parse_duration_ms(s):
    """Parse a Go duration string (e.g. "1m2.5s", "234.5µs", "0s") into ms.

    Returns None if no duration component is found.
    """
    components = DUR_COMPONENT_RE.findall(s)
    if not components:
        return None
    total = 0.0
    for value, unit in components:
        total += float(value) * UNIT_TO_MS[unit]
    return total


def matches_program(pid, program):
    """True if pid belongs to program: pid == program or pid starts with 'program-'."""
    return pid == program or pid.startswith(program + "-")


def iter_lines(files, dir_path):
    """Yield log lines from the given files, from dir_path (recursively), or stdin."""
    paths = list(files)
    if dir_path:
        if not os.path.isdir(dir_path):
            print(f"Error: directory {dir_path!r} does not exist", file=sys.stderr)
            sys.exit(1)
        # Prefer the conventional sigmaos-node-logs subdir if present.
        node_logs = os.path.join(dir_path, "sigmaos-node-logs")
        root = node_logs if os.path.isdir(node_logs) else dir_path
        for path in sorted(glob.glob(os.path.join(root, "**", "*"), recursive=True)):
            if os.path.isfile(path):
                paths.append(path)

    if not paths:
        for line in sys.stdin:
            yield line
        return

    for path in paths:
        try:
            with open(path, "r", errors="replace") as f:
                for line in f:
                    yield line
        except Exception as e:
            print(f"Warning: could not read {path}: {e}", file=sys.stderr)


def collect(lines, program):
    """Group op/sinceSpawn durations (in ms) by log type for the given program.

    Returns a dict: logtype -> {"op": [ms...], "spawn": [ms...]}.
    """
    stats = {}
    for line in lines:
        m = LINE_RE.search(line)
        if not m:
            continue
        if not matches_program(m.group("pid"), program):
            continue
        logtype = m.group("logtype")
        op_ms = parse_duration_ms(m.group("op"))
        spawn_ms = parse_duration_ms(m.group("spawn"))
        entry = stats.setdefault(logtype, {"op": [], "spawn": []})
        if op_ms is not None:
            entry["op"].append(op_ms)
        if spawn_ms is not None:
            entry["spawn"].append(spawn_ms)
    return stats


def percentile(sorted_data, p):
    """Percentile (0-100) via linear interpolation between closest ranks (numpy default)."""
    if not sorted_data:
        return None
    if len(sorted_data) == 1:
        return sorted_data[0]
    rank = (p / 100.0) * (len(sorted_data) - 1)
    lo = int(rank)
    hi = min(lo + 1, len(sorted_data) - 1)
    frac = rank - lo
    return sorted_data[lo] + (sorted_data[hi] - sorted_data[lo]) * frac


def summarize(values):
    """Return (count, avg, median, p90, p95) in ms for a list of values."""
    n = len(values)
    if n == 0:
        return (0, None, None, None, None)
    s = sorted(values)
    avg = sum(s) / n
    return (n, avg, percentile(s, 50), percentile(s, 90), percentile(s, 95))


def print_section(title, metric_key, stats):
    print("=" * 96)
    print(title)
    print("=" * 96)
    header = f"{'Log type':<44} {'count':>7} {'avg':>10} {'median':>10} {'p90':>10} {'p95':>10}"
    print(header)
    print("-" * 96)

    def fmt(v):
        return f"{v:.3f}" if v is not None else "N/A"

    for logtype in sorted(stats.keys()):
        n, avg, med, p90, p95 = summarize(stats[logtype][metric_key])
        if n == 0:
            continue
        print(
            f"{logtype:<44} {n:>7} {fmt(avg):>10} {fmt(med):>10} {fmt(p90):>10} {fmt(p95):>10}"
        )
    print()


def main():
    parser = argparse.ArgumentParser(
        description="Print statistics for each SPAWN_LATENCY log type of a proc program."
    )
    parser.add_argument(
        "--program",
        required=True,
        help="Proc program name (pid prefix), e.g. 'mr-m-wc' for the MR mapper.",
    )
    parser.add_argument(
        "--dir",
        dest="dir_path",
        default=None,
        help="Benchmark output directory to scan recursively for logs "
        "(uses its sigmaos-node-logs subdir if present).",
    )
    parser.add_argument(
        "files",
        nargs="*",
        help="Log files to read. If none are given and --dir is unset, reads stdin.",
    )
    args = parser.parse_args()

    stats = collect(iter_lines(args.files, args.dir_path), args.program)

    if not stats:
        print(
            f"No SPAWN_LATENCY log lines found for program {args.program!r}.",
            file=sys.stderr,
        )
        sys.exit(1)

    print(f"Program: {args.program}")
    print(f"Log types: {len(stats)}")
    print("All durations in milliseconds (ms).")
    print()

    print_section("op (operation duration)", "op", stats)
    print_section("sinceSpawn (elapsed since spawn)", "spawn", stats)


if __name__ == "__main__":
    main()
