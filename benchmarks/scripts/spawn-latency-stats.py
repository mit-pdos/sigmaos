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
import functools
import glob
import os
import re
import sys
from array import array
from concurrent.futures import ProcessPoolExecutor


# Size of the blocks logs are scanned in. Scanning whole blocks (rather than
# line by line) keeps the search loop inside the regex engine, which matters a
# lot when the logs are gigabytes of mostly-irrelevant lines.
CHUNK_SIZE = 8 << 20


def line_re(program):
    """Regex matching "[<program-pid>] <logtype> op:<dur> sinceSpawn:<dur>".

    The program is baked into the pattern so non-matching pids are rejected by
    the regex engine's literal-prefix scan instead of by Python code. The pid
    matches the program exactly or as a "program-<suffix>" prefix. The log type
    is captured non-greedily so it stops at the first " op:".

    Every piece of the pattern is newline-free, so a match stays within a single
    line even though whole blocks of log (not individual lines) are scanned. In
    particular the separators must be horizontal whitespace: plain "\\s+" would
    let a line ending in "[<pid>]" join with the " op:...sinceSpawn:..." of a
    later line, capturing the log text in between as a bogus log type.
    """
    return re.compile(
        r"\[" + re.escape(program) + r"(?:-[^\]\n]*)?\]"
        r"[^\S\n]+(?P<logtype>[^\n]*?)"
        r"[^\S\n]+op:(?P<op>\S+)[^\S\n]+sinceSpawn:(?P<spawn>\S+)"
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


@functools.lru_cache(maxsize=1 << 16)
def parse_duration_ms(s):
    """Parse a Go duration string (e.g. "1m2.5s", "234.5µs", "0s") into ms.

    Returns None if no duration component is found. Memoized, since the same
    duration strings recur often across log lines.
    """
    components = DUR_COMPONENT_RE.findall(s)
    if not components:
        return None
    total = 0.0
    for value, unit in components:
        total += float(value) * UNIT_TO_MS[unit]
    return total


def input_paths(files, dir_path):
    """The log files to read: the given files plus dir_path scanned recursively."""
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
    return paths


def iter_chunks(f):
    """Yield the contents of f in newline-aligned chunks."""
    tail = ""
    while True:
        buf = f.read(CHUNK_SIZE)
        if not buf:
            if tail:
                yield tail
            return
        buf = tail + buf
        end = buf.rfind("\n")
        if end == -1:
            # No newline in sight (e.g. a binary file); flush to bound memory.
            yield buf
            tail = ""
            continue
        yield buf[: end + 1]
        tail = buf[end + 1 :]


def collect_chunks(chunks, program, stats=None):
    """Group op/sinceSpawn durations (in ms) by log type for the given program.

    Returns a dict: logtype -> {"op": array(ms...), "spawn": array(ms...)}.
    """
    if stats is None:
        stats = {}
    regex = line_re(program)
    for chunk in chunks:
        for m in regex.finditer(chunk):
            logtype = m.group("logtype")
            op_ms = parse_duration_ms(m.group("op"))
            spawn_ms = parse_duration_ms(m.group("spawn"))
            entry = stats.get(logtype)
            if entry is None:
                entry = stats[logtype] = {"op": array("d"), "spawn": array("d")}
            if op_ms is not None:
                entry["op"].append(op_ms)
            if spawn_ms is not None:
                entry["spawn"].append(spawn_ms)
    return stats


def collect_path(args):
    """Collect stats from a single log file. Top-level so it can be pickled."""
    path, program = args
    try:
        with open(path, "r", errors="replace") as f:
            return collect_chunks(iter_chunks(f), program)
    except Exception as e:
        print(f"Warning: could not read {path}: {e}", file=sys.stderr)
        return {}


def merge_stats(dst, src):
    for logtype, entry in src.items():
        into = dst.get(logtype)
        if into is None:
            dst[logtype] = entry
            continue
        into["op"].extend(entry["op"])
        into["spawn"].extend(entry["spawn"])
    return dst


def collect(paths, program, jobs):
    """Collect stats from paths (or stdin if empty), using up to jobs processes."""
    if not paths:
        return collect_chunks(iter_chunks(sys.stdin), program)
    jobs = min(jobs, len(paths))
    if jobs <= 1:
        stats = {}
        for path in paths:
            merge_stats(stats, collect_path((path, program)))
        return stats
    stats = {}
    with ProcessPoolExecutor(max_workers=jobs) as pool:
        for s in pool.map(collect_path, [(p, program) for p in paths]):
            merge_stats(stats, s)
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
        "--jobs",
        "-j",
        type=int,
        default=os.cpu_count() or 1,
        help="Number of processes to scan log files with (default: number of CPUs).",
    )
    parser.add_argument(
        "files",
        nargs="*",
        help="Log files to read. If none are given and --dir is unset, reads stdin.",
    )
    args = parser.parse_args()

    stats = collect(input_paths(args.files, args.dir_path), args.program, args.jobs)

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
