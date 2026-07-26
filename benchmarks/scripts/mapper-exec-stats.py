#!/usr/bin/env python3
"""Summarize the instrumentation added to diagnose slow exec -> main for procs.

Companion to spawn-latency-stats.py, which already summarizes every "op:" log
type. This script parses the three non-duration log lines that spawn-latency
stats can't:

  1. Setup.RuntimeInit.rusage — resource usage accumulated by a proc between
     execve and Go main (getrusage counters reset across exec):

       [pid] Setup.RuntimeInit.rusage utime:12ms stime:8ms minflt:4321 majflt:0 nvcsw:12 nivcsw:57

     Compare utime+stime against the Setup.RuntimeInit wall time reported by
     spawn-latency-stats.py: if wall grows while CPU stays flat, procs are
     waiting for a CPU rather than doing more work, and nivcsw (involuntary
     context switches) should grow with it. majflt counts faults that needed
     I/O — for a binary exec'd from binfs, FUSE round trips into procd.

  2. BinFs.exec — how much FUSE work each exec of a binary cost procd:

       [pid] BinFs.exec "prog" nread:12 nbyte:1572864 npresent:12 presentMs:3ms nfetch:0 fetchMs:0s

     nread/nbyte is what the kernel had to page in through procd rather than
     from its own page cache; npresent/presentMs is the per-read
     chunksrv.IsPresent scan; nfetch should be 0 after the first proc for a
     binary on a node.

  3. NODESTATS — per-node samples of CPU pressure/utilization and how many
     procs the node was running:

       NODESTATS interval:500ms cpuUtil:98.7% cpuSome:82.1% memSome:0.0% ... nproc:13

Usage:

    mapper-exec-stats.py --dir benchmarks/results/PHD_THESIS/be_mr_multiplexing_mem1200
    mapper-exec-stats.py --program mr-m-grep --dir <results dir>
"""

import argparse
import os
import re
import sys

from importlib.machinery import SourceFileLoader

# Reuse the duration parsing, percentiles, and log discovery from the existing
# spawn-latency script so both agree on units and on which files to read.
_here = os.path.dirname(os.path.abspath(__file__))
_sl = SourceFileLoader(
    "spawn_latency_stats", os.path.join(_here, "spawn-latency-stats.py")
).load_module()

parse_duration_ms = _sl.parse_duration_ms
iter_lines = _sl.iter_lines
summarize = _sl.summarize
matches_program = _sl.matches_program

RUSAGE_RE = re.compile(
    r"\[(?P<pid>[^\]]+)\]\s+Setup\.RuntimeInit\.rusage\s+"
    r"cpu:(?P<cpu>\S+)\s+utime:(?P<utime>\S+)\s+stime:(?P<stime>\S+)\s+"
    r"trampCPU:(?P<tramp>\S+)\s+"
    r"minflt:(?P<minflt>\d+)\s+majflt:(?P<majflt>\d+)\s+"
    r"nvcsw:(?P<nvcsw>\d+)\s+nivcsw:(?P<nivcsw>\d+)"
)

BINFS_RE = re.compile(
    r"\[(?P<pid>[^\]]+)\]\s+BinFs\.exec\s+(?P<prog>\S+)\s+"
    r"nread:(?P<nread>\d+)\s+nbyte:(?P<nbyte>\d+)\s+"
    r"npresent:(?P<npresent>\d+)\s+presentMs:(?P<present>\S+)\s+"
    r"nfetch:(?P<nfetch>\d+)\s+fetchMs:(?P<fetch>\S+)"
)

NODESTATS_RE = re.compile(
    r"NODESTATS\s+interval:(?P<interval>\S+)\s+cpuUtil:(?P<cpuUtil>[\d.]+)%\s+"
    r"cpuSome:(?P<cpuSome>[\d.]+)%\s+memSome:(?P<memSome>[\d.]+)%\s+"
    r"memFull:(?P<memFull>[\d.]+)%\s+ioSome:(?P<ioSome>[\d.]+)%\s+"
    r"ioFull:(?P<ioFull>[\d.]+)%\s+loadavg:(?P<loadavg>\S+)\s+"
    r"memAvailMB:(?P<memAvail>\d+)(?:\s+nproc:(?P<nproc>\d+))?"
)


def fmt(v, prec=3):
    return f"{v:.{prec}f}" if v is not None else "N/A"


def print_table(title, rows):
    """rows: list of (label, values). Prints count/avg/median/p90/p95."""
    print("=" * 96)
    print(title)
    print("=" * 96)
    print(f"{'Metric':<28} {'count':>7} {'avg':>12} {'median':>12} {'p90':>12} {'p95':>12}")
    print("-" * 96)
    for label, values in rows:
        n, avg, med, p90, p95 = summarize(values)
        if n == 0:
            continue
        print(
            f"{label:<28} {n:>7} {fmt(avg):>12} {fmt(med):>12} {fmt(p90):>12} {fmt(p95):>12}"
        )
    print()


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument(
        "--program",
        default="mr-m-",
        help="Proc program name (pid prefix) to filter on; default 'mr-m-' (MR mappers).",
    )
    ap.add_argument(
        "--dir",
        dest="dir_path",
        default=None,
        help="Benchmark output directory to scan recursively for logs.",
    )
    ap.add_argument("files", nargs="*", help="Log files; default stdin.")
    args = ap.parse_args()

    ru = {k: [] for k in ("cpu", "utime", "stime", "tramp", "minflt", "majflt", "nvcsw", "nivcsw")}
    binfs = {k: [] for k in ("nread", "kbyte", "npresent", "presentMs", "nfetch", "fetchMs")}
    node = {k: [] for k in ("cpuUtil", "cpuSome", "memSome", "memFull", "ioSome", "ioFull", "memAvail", "nproc")}
    nprog = {}

    for line in iter_lines(args.files, args.dir_path):
        m = RUSAGE_RE.search(line)
        if m and matches_program(m.group("pid"), args.program):
            # cpu is exec -> main only (the trampoline's CPU, reported as
            # trampCPU, has already been subtracted).
            ru["cpu"].append(parse_duration_ms(m.group("cpu")) or 0.0)
            ru["utime"].append(parse_duration_ms(m.group("utime")) or 0.0)
            ru["stime"].append(parse_duration_ms(m.group("stime")) or 0.0)
            ru["tramp"].append(parse_duration_ms(m.group("tramp")) or 0.0)
            for k in ("minflt", "majflt", "nvcsw", "nivcsw"):
                ru[k].append(float(m.group(k)))
            continue
        m = BINFS_RE.search(line)
        if m:
            prog = m.group("prog").strip('"')
            nprog[prog] = nprog.get(prog, 0) + 1
            if matches_program(m.group("pid"), args.program) or args.program == "":
                binfs["nread"].append(float(m.group("nread")))
                binfs["kbyte"].append(float(m.group("nbyte")) / 1024.0)
                binfs["npresent"].append(float(m.group("npresent")))
                binfs["presentMs"].append(parse_duration_ms(m.group("present")) or 0.0)
                binfs["nfetch"].append(float(m.group("nfetch")))
                binfs["fetchMs"].append(parse_duration_ms(m.group("fetch")) or 0.0)
            continue
        m = NODESTATS_RE.search(line)
        if m:
            for k in ("cpuUtil", "cpuSome", "memSome", "memFull", "ioSome", "ioFull"):
                node[k].append(float(m.group(k)))
            node["memAvail"].append(float(m.group("memAvail")))
            if m.group("nproc") is not None:
                node["nproc"].append(float(m.group("nproc")))

    if not any(v for v in ru.values()) and not any(v for v in binfs.values()) and not any(v for v in node.values()):
        print("No instrumentation lines found. Was the build with the new "
              "instrumentation pushed, and SPAWN_LAT enabled?", file=sys.stderr)
        return 1

    print(f"Program filter: {args.program}\n")

    print_table(
        "exec -> main resource usage (ms, and counts) — is it work, or waiting for CPU?",
        [
            ("CPU exec->main ms", ru["cpu"]),
            ("utime ms (incl tramp)", ru["utime"]),
            ("stime ms (incl tramp)", ru["stime"]),
            ("trampoline CPU ms", ru["tramp"]),
            ("minor faults", ru["minflt"]),
            ("major faults", ru["majflt"]),
            ("vol ctx switches", ru["nvcsw"]),
            ("invol ctx switches", ru["nivcsw"]),
        ],
    )

    print_table(
        "binfs FUSE work per exec — how much of the binary came through procd",
        [
            ("FUSE reads", binfs["nread"]),
            ("KB read through FUSE", binfs["kbyte"]),
            ("IsPresent calls", binfs["npresent"]),
            ("IsPresent ms", binfs["presentMs"]),
            ("chunk fetches", binfs["nfetch"]),
            ("chunk fetch ms", binfs["fetchMs"]),
        ],
    )
    if nprog:
        print("BinFs.exec lines per program:")
        for prog, n in sorted(nprog.items(), key=lambda kv: -kv[1]):
            print(f"  {prog:<60} {n}")
        print()

    print_table(
        "node state (per 500ms sample, all nodes pooled) — %, MB, proc count",
        [
            ("cpuUtil %", node["cpuUtil"]),
            ("cpuSome (PSI stall) %", node["cpuSome"]),
            ("memSome %", node["memSome"]),
            ("memFull %", node["memFull"]),
            ("ioSome %", node["ioSome"]),
            ("ioFull %", node["ioFull"]),
            ("MemAvailable MB", node["memAvail"]),
            ("procs running/node", node["nproc"]),
        ],
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
