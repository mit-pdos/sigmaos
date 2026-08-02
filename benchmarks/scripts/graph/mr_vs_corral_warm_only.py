#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import glob
import os
import re
import statistics
import sys
import durationpy

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

# One entry per configuration the graph can show. Each becomes an optional
# --<key>_dir argument holding that run's results directory; only the ones
# supplied on the command line are plotted, in this order. "kind" selects how
# the run's execution time is scraped, "family" picks the bar color so that the
# fslib/getput/cosandbox/lambda groups are distinguishable at a glance, and
# "source" is where the job read its input, which --source filters on.
CONFIGS = [
  ("ux",                   "UX\nfslib",       "sigma",  "fslib",     "ux"),
  ("ux_getput_mapper",     "UX\ngetput\nmap", "sigma",  "getput",    "ux"),
  ("ux_getput_reducer",    "UX\ngetput\nred", "sigma",  "getput",    "ux"),
  ("ux_getput_both",       "UX\ngetput\nboth", "sigma", "getput",    "ux"),
  ("ux_cosandbox_mapper",  "UX\ncosbx\nmap",  "sigma",  "cosandbox", "ux"),
  ("ux_cosandbox_reducer", "UX\ncosbx\nred",  "sigma",  "cosandbox", "ux"),
  ("ux_cosandbox_both",    "UX\ncosbx\nboth", "sigma",  "cosandbox", "ux"),
  ("s3",                   "S3\nfslib",       "sigma",  "fslib",     "s3"),
  ("s3_getput_mapper",     "S3\ngetput\nmap", "sigma",  "getput",    "s3"),
  ("s3_getput_reducer",    "S3\ngetput\nred", "sigma",  "getput",    "s3"),
  ("s3_getput_both",       "S3\ngetput\nboth", "sigma", "getput",    "s3"),
  ("s3_cosandbox_mapper",  "S3\ncosbx\nmap",  "sigma",  "cosandbox", "s3"),
  ("s3_cosandbox_reducer", "S3\ncosbx\nred",  "sigma",  "cosandbox", "s3"),
  ("s3_cosandbox_both",    "S3\ncosbx\nboth", "sigma",  "cosandbox", "s3"),
  ("corral",               "λ-mr",            "corral", "lambda",    "lambda"),
]

SOURCES = ["ux", "s3", "lambda"]

FAMILY_COLORS = {
  "fslib":     "C0",
  "getput":    "C1",
  "cosandbox": "C2",
  "lambda":    "C3",
}

FAMILY_LABELS = {
  "fslib":     "σOS-mr (fslib)",
  "getput":    "σOS-mr (get/put)",
  "cosandbox": "σOS-mr (co-sandbox)",
  "lambda":    "λ-mr",
}

def bench_out(dname):
  # The benchmark names its output file after the number of nodes in the run
  # (bench.out.17, bench.out.0, ...), so glob rather than assuming a suffix.
  outs = sorted(glob.glob(os.path.join(dname, "bench.out.*")))
  if len(outs) == 0:
    return None
  return outs[0]

def run_dirs(dname):
  # A configuration's results directory either holds a single run's output
  # directly (the old layout) or one subdirectory per repetition (run-1,
  # run-2, ...), which is what the benchmark writes when it repeats a
  # configuration. Accept both.
  if bench_out(dname) is not None:
    return [dname]
  subdirs = sorted(d for d in glob.glob(os.path.join(dname, "*"))
                   if os.path.isdir(d) and bench_out(d) is not None)
  return subdirs

def scrape_time(dname, sigma):
  fn = bench_out(dname)
  if fn is None:
    return None
  with open(fn, "r") as f:
    b = f.read()
  lines = b.split("\n")
  if sigma:
    lines = [ l for l in lines if "Mean:" in l ]
  else:
    lines = [ l.strip() for l in lines if "Job Execution Time:" in l ]
  if len(lines) == 0:
    return None
  t_str = lines[0].split(" ")[-1]
  t = durationpy.from_str(t_str)
  return t.total_seconds()

def scrape_times(dname, sigma):
  # Every run's execution time under this configuration's results directory.
  ts = [ scrape_time(d, sigma) for d in run_dirs(dname) ]
  return [ t for t in ts if t is not None ]

# The per-task stats block names each task and then reports its times:
#   [mr-m-wc-<job>-<pid>, kid:sigma-...]:
#       in 216 MB out 19 MB tot ... inner 7166ms outer 7965ms gets 5997ms (n 86) ...
# mr-m-* is a mapper, mr-r-* a reducer.
TASK_RE = re.compile(r'^\[(mr-[mr])-')
GETS_RE = re.compile(r'\bgets (\d+)ms')
PHASE_RE = re.compile(r'map phase (\d+)ms reduce phase (\d+)ms')

def scrape_run_stats(dname):
  # A run's median mapper/reducer gets time and its phase durations, in ms; any
  # of them None if the run's output doesn't carry it (a corral run carries
  # none). Medians rather than means: straggler tails inflate the mean by ~30%.
  fn = bench_out(dname)
  if fn is None:
    return {}
  with open(fn, "r") as f:
    lines = f.read().split("\n")
  gets = {"mr-m": [], "mr-r": []}
  kind = None
  st = {}
  for l in lines:
    m = TASK_RE.match(l)
    if m:
      kind = m.group(1)
      continue
    if kind is not None:
      g = GETS_RE.search(l)
      if g:
        gets[kind].append(int(g.group(1)))
        kind = None
        continue
    p = PHASE_RE.search(l)
    if p and "map_ms" not in st:
      st["map_ms"] = int(p.group(1))
      st["reduce_ms"] = int(p.group(2))
  for k, name in (("mr-m", "map_gets_ms"), ("mr-r", "reduce_gets_ms")):
    if gets[k]:
      st[name] = int(statistics.median(gets[k]))
      st[name + "_n"] = len(gets[k])
  return st

def fmt_ms(st, key):
  return "%6s" % (st[key] if key in st else "-")

def report_runs(label, dirs, times):
  # Print what each run contributed, so that a config's mean and error bar can
  # be traced back to individual runs, and so the input-read cost (gets) and the
  # phase split are visible next to the end-to-end number they explain.
  print("%s (%d run%s)" % (label.replace("\n", " "), len(dirs), "" if len(dirs) == 1 else "s"))
  rows = []
  for d, t in zip(dirs, times):
    st = scrape_run_stats(d)
    rows.append(st)
    print("    %-10s e2e %6.2fs  map %s  reduce %s  gets/mapper %s  gets/reducer %s"
          % (os.path.basename(d.rstrip("/")), t, fmt_ms(st, "map_ms"), fmt_ms(st, "reduce_ms"),
             fmt_ms(st, "map_gets_ms"), fmt_ms(st, "reduce_gets_ms")))
  if len(rows) > 1:
    med = {}
    for k in ("map_ms", "reduce_ms", "map_gets_ms", "reduce_gets_ms"):
      v = [ r[k] for r in rows if k in r ]
      if v:
        med[k] = int(statistics.median(v))
    print("    %-10s e2e %6.2fs  map %s  reduce %s  gets/mapper %s  gets/reducer %s"
          % ("median", statistics.median(times), fmt_ms(med, "map_ms"), fmt_ms(med, "reduce_ms"),
             fmt_ms(med, "map_gets_ms"), fmt_ms(med, "reduce_gets_ms")))

def collect(args):
  # The (label, family, mean, stddev, nrun) of every configuration which was
  # both supplied and has at least one scrapable run, in CONFIGS order. The
  # error bar is the sample standard deviation across runs, and is zero for a
  # configuration which was only run once.
  bars = []
  for key, label, kind, family, source in CONFIGS:
    dname = getattr(args, key + "_dir")
    if dname is None:
      continue
    # --source restricts the graph to configurations reading from one input
    # source, so that a directory list naming everything can be reused as-is.
    if args.source is not None and source not in args.source:
      continue
    dirs = run_dirs(dname)
    ts = scrape_times(dname, kind == "sigma")
    if len(ts) == 0:
      print("Warning: no benchmark output in %s; skipping %s" % (dname, key), file=sys.stderr)
      continue
    report_runs(label, dirs, ts)
    sd = float(np.std(ts, ddof=1)) if len(ts) > 1 else 0.0
    bars.append((label, family, float(np.mean(ts)), sd, len(ts)))
  return bars

def setup_graph(nbar):
  # Widen with the number of bars so the tick labels stay legible.
  fig, ax = plt.subplots(figsize=(max(6.4, 0.85 * nbar + 1.0), 3.2))
  ax.set_ylabel("Execution Time (seconds)")
  return fig, ax

def graph_data(args):
  bars = collect(args)
  if len(bars) == 0:
    if args.source is not None:
      print("Error: no configurations with results match --source %s" % ",".join(args.source), file=sys.stderr)
    else:
      print("Error: no configurations supplied (pass at least one --<config>_dir)", file=sys.stderr)
    sys.exit(1)

  fig, ax = setup_graph(len(bars))

  x = np.arange(len(bars))
  times = [ t for _, _, t, _, _ in bars ]
  errs = [ e for _, _, _, e, _ in bars ]
  colors = [ FAMILY_COLORS[f] for _, f, _, _, _ in bars ]
  plt.bar(x, times, width=0.7, color=colors, yerr=errs, capsize=3,
          error_kw={"ecolor": "black", "elinewidth": 1})
  top = max(t + e for t, e in zip(times, errs))
  for i, (_, _, v, e, n) in enumerate(bars):
    txt = str(round(v, 2)) if n < 2 else "%.2f±%.2f" % (v, e)
    plt.text(x[i], v + e + top * 0.02, txt, ha="center", fontsize=8)
  ax.set_xticks(x)
  ax.set_xticklabels([ l for l, _, _, _, _ in bars ], fontsize=7)
  ax.set_ylim(bottom=0, top=top * 1.25)
  ax.tick_params(axis='x', bottom=False)

  nruns = set(n for _, _, _, _, n in bars)
  if nruns != {1}:
    ax.set_ylabel("Execution Time (seconds)\nmean of %s runs, ±1 s.d." %
                  ("/".join(str(n) for n in sorted(nruns))))

  # One legend entry per family which actually appears.
  families = []
  for _, f, _, _, _ in bars:
    if f not in families:
      families.append(f)
  handles = [ matplotlib.patches.Patch(color=FAMILY_COLORS[f], label=FAMILY_LABELS[f]) for f in families ]
  ax.legend(handles=handles, loc="upper left", fontsize=8, ncol=len(families))

  if args.title is not None:
    title = args.title
  elif args.app == "grep":
    title = "MapReduce Grep Execution Time"
  elif args.app == "wc":
    title = "MapReduce WordCount Execution Time"
  else:
    assert(False)
  plt.title(title)
  fig.tight_layout()
  fig.savefig(args.out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  for key, label, _, _, _ in CONFIGS:
    parser.add_argument("--" + key + "_dir", type=str, default=None,
                        help="Results directory for the %s configuration" % label.replace("\n", " "))
  parser.add_argument("--source", type=str, action="append", choices=SOURCES, default=None,
                      help="Only graph configurations reading from this input source. "
                           "Repeatable; omit to graph every supplied configuration.")
  parser.add_argument("--app", type=str, default="wc")
  parser.add_argument("--title", type=str, default=None)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args)
