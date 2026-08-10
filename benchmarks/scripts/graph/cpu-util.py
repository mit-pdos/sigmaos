#!/usr/bin/env python3
"""Graph the CPU used by each infrastructure service over the course of a run.

The kernel's CPU monitor (kernel/cpumon.go, on when SIGMADEBUG includes CPU_MON)
prints one line per node per 100ms round to that node's log:

    11:53:06.892676 sigma-sigma900-27b CPU_MON node 1.9/4 cores over 278ms; \
sampled 0.9 over 172 procs (5 new); unsampled 1.1; procd 0.3; sigma 0.2 \
[bootkernel 0.0 knamed 0.0 msched 0.0]; platform 0.3 [containerd 0.1 etcd 0.0]; \
procs 0.1 [dbus-daemon 0.0 systemd 0.0]

"node X/N" is the whole node's utilization out of its N cores, "procd" is procd's
own processes, "sigma" breaks down sigmaOS's servers by command name, "platform"
the daemons underneath it (containerd, dockerd, etcd), and "procs" everything
else, which on a worker node is the user procs. "unsampled" is the part of the
node total that no per-process sample caught -- procs too short-lived to be seen
by two consecutive rounds, which a fine-grained MR job produces a lot of.

This script sums each of those across every node in a run and plots them as a
stacked area over time, in cores, plus a summary table on stdout. The indented
per-cgroup lines the monitor also prints are ignored: they break a single node's
procd cgroups down further, which is a per-node question, not a cluster one.

    cpu-util.py --measurement_dir <run dir> --out cpu_util.pdf

Time is measured from the start of the benchmark itself (the first sample in the
first test-*-tpt.out), so the x-axis lines up with the throughput graphs; the
kernels start monitoring before that, so early samples have negative times and
are dropped unless --xmin says otherwise.

A job that dedicated machines to serving its data (--n_dedicated_ux_nodes) gets
fsuxd reported once for those machines and once for the rest, since summing the
two hides what dedicating them cost. Which machines those were is read from the
run's bench.out; see --split_dedicated and --dedicated_kernels.
"""

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import datetime
import glob
import os
import re
import sys

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

# A node line: everything up to the first per-group breakdown. The indented
# per-cgroup lines don't match, because they carry a label where "node" is.
NODE_RE = re.compile(
    r"^(?P<h>\d\d):(?P<m>\d\d):(?P<s>\d\d\.\d+)\s+\S+\s+CPU_MON node\s+"
    r"(?P<cores>[-\d.]+)/(?P<ncores>[\d.]+) cores over (?P<dt>[\d.]+)ms;"
)

# Each "<group> <total> [<label> <cores> ...]" section of a node line. "procd"
# and "unsampled" have no bracketed breakdown and are read separately.
GROUP_RE = re.compile(r"(?P<group>sigma|platform|procs)\s+[-\d.]+\s+\[(?P<body>[^\]]*)\]")
PROCD_RE = re.compile(r"procd\s+(?P<cores>[-\d.]+);")
UNSAMPLED_RE = re.compile(r"unsampled\s+(?P<cores>[-\d.]+);")

# "other(x7)" from the monitor's top-N truncation of the "procs" group.
LABEL_RE = re.compile(r"^(?P<label>.+?)(?:\(x\d+\))?$")

# Everything not attributed to a named service is the workload, not
# infrastructure: the procs themselves, plus the samples too short-lived to
# catch. Kept apart from the per-service series so the infrastructure cost can be
# read on its own, and drawn only with --include_procs.
PROCS_LABEL = "procs (workload)"
UNSAMPLED_LABEL = "unsampled (short-lived procs)"
WORKLOAD_LABELS = (PROCS_LABEL, UNSAMPLED_LABEL)

# The line MRJobInstance.DedicateUxNodes logs (at ALWAYS, so it lands in
# bench.out) naming the machines it set aside to serve the job's data.
DEDICATED_RE = re.compile(
    r"Dedicating \d+ machines to hosting MR input: \[(?P<kids>[^\]]*)\]"
)

# Which services to report separately on the dedicated machines. fsuxd by
# default because serving data is the whole reason those machines were set
# aside: summed with the fsuxd running on every other node, the number that
# answers "what did dedicating them cost" is invisible.
DEFAULT_SPLIT = "fsuxd"


def node_key(path):
    """The node a log file or kernel ID belongs to, e.g. "sigma900".

    Log files are named "<node>-<ec2 hostname>.out" and kernel IDs are
    "sigma-<node>-<rand>", so both reduce to the same key. Matching on the whole
    component matters: "sigma900" must not match "sigma9000".
    """
    name = os.path.basename(path)
    parts = name.split("-")
    if len(parts) >= 2 and parts[0] == "sigma":
        # A kernel ID.
        return parts[1]
    return parts[0]


def dedicated_nodes(measurement_dir, override):
    """The nodes dedicated to serving the job's data, as node keys.

    Read from the benchmark's own output unless override (a comma-separated list
    of kernel IDs or node names) says otherwise. Returns an empty set if the run
    dedicated no machines, or if its bench.out isn't there to say.
    """
    if override:
        return {node_key(k.strip()) for k in override.split(",") if k.strip()}
    kids = []
    for path in sorted(glob.glob(os.path.join(measurement_dir, "bench.out*"))):
        try:
            with open(path, "r", errors="replace") as f:
                for line in f:
                    m = DEDICATED_RE.search(line)
                    if m:
                        # A run may dedicate more than once (one job per realm);
                        # every machine named is dedicated.
                        kids.extend(m.group("kids").split())
        except Exception as e:
            print("Warning: could not read {}: {}".format(path, e), file=sys.stderr)
    return {node_key(k) for k in kids}


def parse_node_line(line):
    """Parse one CPU_MON node line into (time_of_day_sec, dt_sec, ncores, {label: cores}).

    Returns None if the line isn't a node line.
    """
    m = NODE_RE.match(line)
    if m is None:
        return None
    tod = int(m.group("h")) * 3600 + int(m.group("m")) * 60 + float(m.group("s"))
    dt = float(m.group("dt")) / 1000.0
    ncores = float(m.group("ncores"))

    by_label = {}
    pm = PROCD_RE.search(line)
    if pm:
        by_label["procd"] = float(pm.group("cores"))
    um = UNSAMPLED_RE.search(line)
    if um:
        by_label[UNSAMPLED_LABEL] = float(um.group("cores"))
    for gm in GROUP_RE.finditer(line):
        toks = gm.group("body").split()
        # The body is "<label> <cores>" pairs; labels never contain spaces.
        for i in range(0, len(toks) - 1, 2):
            label = LABEL_RE.match(toks[i]).group("label")
            if gm.group("group") == "procs":
                # Individual non-sigmaOS commands (systemd, the user procs' own
                # binaries, ...) are noise at cluster scale; what matters is that
                # they aren't infrastructure.
                label = PROCS_LABEL
            try:
                v = float(toks[i + 1])
            except ValueError:
                continue
            by_label[label] = by_label.get(label, 0.0) + v
    return (tod, dt, ncores, by_label)


def read_node_logs(log_dir, dedicated=frozenset(), split_labels=frozenset()):
    """Parse every node log in log_dir. Returns (samples, node_ncores, ndedicated).

    samples is a list of (time_of_day_sec, dt_sec, {label: cores}); node_ncores
    maps log file -> the node's core count; ndedicated is how many of the nodes
    that reported are dedicated ones.

    A label in split_labels is reported separately on the dedicated machines,
    which is the only place the split can be made: the monitor's line says what a
    service cost on its node, and the node is the thing that was dedicated.
    """
    samples = []
    node_ncores = {}
    seen_dedicated = set()
    paths = sorted(glob.glob(os.path.join(log_dir, "*")))
    for path in paths:
        if not os.path.isfile(path):
            continue
        is_dedicated = node_key(path) in dedicated
        try:
            with open(path, "r", errors="replace") as f:
                for line in f:
                    # Cheap reject: the logs are hundreds of MB and only a small
                    # fraction of lines are ours.
                    if "CPU_MON node" not in line:
                        continue
                    parsed = parse_node_line(line)
                    if parsed is None:
                        continue
                    tod, dt, ncores, by_label = parsed
                    if split_labels:
                        by_label = relabel_split(by_label, split_labels, is_dedicated)
                    samples.append((tod, dt, by_label))
                    node_ncores[path] = max(node_ncores.get(path, 0.0), ncores)
                    if is_dedicated:
                        seen_dedicated.add(node_key(path))
        except Exception as e:
            print("Warning: could not read {}: {}".format(path, e), file=sys.stderr)
    return samples, node_ncores, len(seen_dedicated)


def relabel_split(by_label, split_labels, is_dedicated):
    """Tag the split labels of one sample as dedicated or colocated."""
    out = {}
    for label, cores in by_label.items():
        if label in split_labels:
            label = "{} ({})".format(
                label, "dedicated" if is_dedicated else "colocated"
            )
        out[label] = cores
    return out


def bench_start_time_of_day(measurement_dir):
    """Time of day (in seconds, UTC) at which the benchmark itself started.

    Taken from the first sample of the run's test-*-tpt.out, whose timestamps are
    microseconds since the epoch. Returns None if there is no such file.
    """
    paths = sorted(glob.glob(os.path.join(measurement_dir, "test-*-tpt.out")))
    for path in paths:
        with open(path, "r") as f:
            for line in f:
                line = line.strip()
                if not line or "us," not in line:
                    continue
                epoch_us = float(line.split("us,")[0])
                # The node logs timestamp in UTC.
                t = datetime.datetime.fromtimestamp(
                    epoch_us / 1e6, datetime.timezone.utc
                )
                return t.hour * 3600 + t.minute * 60 + t.second + t.microsecond / 1e6
    return None


def to_relative_times(samples, t0):
    """Rewrite sample times to seconds relative to t0, handling a midnight wrap."""
    out = []
    for tod, dt, by_label in samples:
        t = tod - t0
        # The logs carry a time of day, not a date, so a run spanning midnight
        # would otherwise jump back a day.
        if t < -12 * 3600:
            t += 24 * 3600
        elif t > 12 * 3600:
            t -= 24 * 3600
        out.append((t, dt, by_label))
    return out


def bucketize(samples, step, xmin, xmax):
    """Average cores per label over each step-second window, summed across nodes.

    Each sample reports the cores a label used over its own dt, so weighting by dt
    and dividing by the window gives the window's mean cores: the ~10 samples a
    node emits per second add up to that node's contribution, not ten times it.

    Returns (times, {label: cores array}).
    """
    buckets = {}
    for t, dt, by_label in samples:
        if xmin is not None and t < xmin:
            continue
        if xmax is not None and t > xmax:
            continue
        b = int((t - (xmin or 0.0)) // step)
        entry = buckets.setdefault(b, {})
        for label, cores in by_label.items():
            entry[label] = entry.get(label, 0.0) + cores * dt
    if not buckets:
        return np.array([]), {}
    labels = sorted({l for e in buckets.values() for l in e})
    idxs = sorted(buckets.keys())
    times = np.array([(xmin or 0.0) + i * step for i in idxs])
    series = {}
    for label in labels:
        series[label] = np.array([buckets[i].get(label, 0.0) / step for i in idxs])
    return times, series


def order_labels(series, include_procs, top):
    """Labels to plot, biggest total first, with the tail summed into "other"."""
    labels = [l for l in series if include_procs or l not in WORKLOAD_LABELS]
    labels.sort(key=lambda l: series[l].sum(), reverse=True)
    # A label that used no CPU at all in the window is not worth a legend entry.
    labels = [l for l in labels if series[l].sum() > 0]
    if top > 0 and len(labels) > top:
        rest = labels[top:]
        labels = labels[:top]
        series["other"] = sum(series[l] for l in rest)
        labels.append("other")
    return labels


def print_summary(times, series, labels, step, capacity):
    """Print mean/peak cores and core-seconds per service over the window."""
    span = len(times) * step
    print("Window: {:.1f}s ({} x {:.1f}s buckets)".format(span, len(times), step))
    if capacity > 0:
        print("Cluster capacity: {:.0f} cores".format(capacity))
    print()
    header = "{:<32} {:>10} {:>10} {:>14} {:>10}".format(
        "Service", "mean", "peak", "core-sec", "% cluster"
    )
    print(header)
    print("-" * len(header))
    for label in labels:
        y = series[label]
        pct = 100.0 * y.mean() / capacity if capacity > 0 else float("nan")
        print(
            "{:<32} {:>10.2f} {:>10.2f} {:>14.1f} {:>10.2f}".format(
                label, y.mean(), y.max(), y.sum() * step, pct
            )
        )
    total = sum(series[l] for l in labels)
    pct = 100.0 * total.mean() / capacity if capacity > 0 else float("nan")
    print("-" * len(header))
    print(
        "{:<32} {:>10.2f} {:>10.2f} {:>14.1f} {:>10.2f}".format(
            "TOTAL", total.mean(), total.max(), total.sum() * step, pct
        )
    )
    print()


def graph(times, series, labels, out, title, capacity, ylabel):
    fig, ax = plt.subplots(1, figsize=(6.4, 3.2))
    # tab20 gives enough distinguishable colors for the full service list.
    cmap = plt.get_cmap("tab20")
    colors = [cmap(i % 20) for i in range(len(labels))]
    ax.stackplot(times, [series[l] for l in labels], labels=labels, colors=colors)
    ax.set_xlabel("Time (sec)")
    ax.set_ylabel(ylabel)
    ax.set_xlim(left=times[0], right=times[-1])
    ax.set_ylim(bottom=0)
    if capacity > 0:
        ax.axhline(capacity, color="black", linestyle=":", linewidth=1.0)
    if title:
        ax.set_title(title)
    ncol = 2 if len(labels) > 8 else 1
    ax.legend(loc="upper left", bbox_to_anchor=(1.01, 1.0), ncol=ncol, fontsize=7)
    ax.grid(which="major", linestyle="--", linewidth=0.5, alpha=0.7)
    ax.set_axisbelow(True)
    fig.savefig(out, bbox_inches="tight")


def main():
    parser = argparse.ArgumentParser(
        description="Graph per-service CPU utilization from a run's CPU_MON logs."
    )
    parser.add_argument("--measurement_dir", type=str, required=True,
                        help="Benchmark result dir (containing sigmaos-node-logs).")
    parser.add_argument("--out", type=str, required=True, help="Output graph path.")
    parser.add_argument("--title", type=str, default="")
    parser.add_argument("--step_size", type=float, default=1.0,
                        help="Averaging window, in seconds (default: 1).")
    parser.add_argument("--xmin", type=float, default=0.0,
                        help="Start of the window, in seconds since the benchmark "
                        "started (default: 0, i.e. drop kernel-startup samples).")
    parser.add_argument("--xmax", type=float, default=None,
                        help="End of the window, in seconds (default: end of run).")
    parser.add_argument("--include_procs", action="store_true", default=False,
                        help="Also plot the workload's own CPU (the procs, and the "
                        "unsampled remainder), not just the infrastructure.")
    parser.add_argument("--top", type=int, default=14,
                        help="Plot at most this many services individually, summing "
                        "the rest into 'other' (default: 14; 0 for all).")
    parser.add_argument("--split_dedicated", type=str, default=DEFAULT_SPLIT,
                        help="Comma-separated services to report separately on the "
                        "machines the job dedicated to serving its data (default: "
                        "{}; empty to report cluster-wide totals only).".format(
                            DEFAULT_SPLIT))
    parser.add_argument("--dedicated_kernels", type=str, default=None,
                        help="Comma-separated kernel IDs or node names that were "
                        "dedicated, overriding what the run's bench.out says.")
    args = parser.parse_args()

    log_dir = os.path.join(args.measurement_dir, "sigmaos-node-logs")
    if not os.path.isdir(log_dir):
        log_dir = args.measurement_dir
    split_labels = {s.strip() for s in args.split_dedicated.split(",") if s.strip()}
    dedicated = dedicated_nodes(args.measurement_dir, args.dedicated_kernels)
    if split_labels and not dedicated:
        # Nothing to split against, so don't imply there was: leave the labels
        # as the cluster-wide totals they are.
        split_labels = set()
    samples, node_ncores, ndedicated = read_node_logs(log_dir, dedicated, split_labels)
    if not samples:
        print("No CPU_MON node lines found in {}. Was the run made with CPU_MON "
              "in SIGMADEBUG?".format(log_dir), file=sys.stderr)
        sys.exit(1)

    t0 = bench_start_time_of_day(args.measurement_dir)
    if t0 is None:
        # No benchmark tpt file, so measure from the first sample instead.
        t0 = min(s[0] for s in samples)
        print("Warning: no test-*-tpt.out in {}; times are relative to the first "
              "CPU_MON sample".format(args.measurement_dir), file=sys.stderr)
    samples = to_relative_times(samples, t0)

    capacity = sum(node_ncores.values())
    times, series = bucketize(samples, args.step_size, args.xmin, args.xmax)
    if len(times) == 0:
        print("No samples in [{}, {}]".format(args.xmin, args.xmax), file=sys.stderr)
        sys.exit(1)
    labels = order_labels(series, args.include_procs, args.top)

    if dedicated:
        print("Nodes: {} ({} dedicated to serving data, {} running procs)".format(
            len(node_ncores), ndedicated, len(node_ncores) - ndedicated))
        missing = dedicated - {node_key(p) for p in node_ncores}
        if missing:
            # Their fsuxd is running and serving either way; without their logs
            # its cost simply isn't in these numbers.
            print("Warning: {} dedicated node(s) reported no CPU_MON samples: "
                  "{}".format(len(missing), " ".join(sorted(missing))),
                  file=sys.stderr)
    else:
        print("Nodes: {}".format(len(node_ncores)))
    print_summary(times, series, labels, args.step_size, capacity)

    ylabel = "Cores" if args.include_procs else "Infra. cores"
    graph(times, series, labels, args.out, args.title, capacity, ylabel)
    print("Wrote {}".format(args.out))


if __name__ == "__main__":
    main()
