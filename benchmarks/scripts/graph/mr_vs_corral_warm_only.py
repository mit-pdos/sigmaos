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
  ("corral",               "λ-mr",                            "corral", "lambda",    "lambda"),
  ("s3",                   "σOS-mr (S3)",                     "sigma",  "fslib",     "s3"),
  ("ux",                   "σOS-mr (UX)",                     "sigma",  "fslib",     "ux"),
  ("ux_getput_mapper",     "σOS-mr get/put map (UX)",         "sigma",  "getput",    "ux"),
  ("ux_getput_reducer",    "σOS-mr get/put red (UX)",         "sigma",  "getput",    "ux"),
  ("ux_getput_both",       "σOS-mr get/put (UX)",             "sigma",  "getput",    "ux"),
  ("ux_cosandbox_mapper",  "σOS-mr co-sandbox map (UX)",      "sigma",  "cosandbox", "ux"),
  ("ux_cosandbox_reducer", "σOS-mr co-sandbox red (UX)",      "sigma",  "cosandbox", "ux"),
  ("ux_cosandbox_both",    "σOS-mr co-sandbox (UX)",          "sigma",  "cosandbox", "ux"),
  ("s3_getput_mapper",     "σOS-mr get/put map (S3)",         "sigma",  "getput",    "s3"),
  ("s3_getput_reducer",    "σOS-mr get/put red (S3)",         "sigma",  "getput",    "s3"),
  ("s3_getput_both",       "σOS-mr get/put (S3)",             "sigma",  "getput",    "s3"),
  ("s3_cosandbox_mapper",  "σOS-mr co-sandbox map (S3)",      "sigma",  "cosandbox", "s3"),
  ("s3_cosandbox_reducer", "σOS-mr co-sandbox red (S3)",      "sigma",  "cosandbox", "s3"),
  ("s3_cosandbox_both",    "σOS-mr (S3) co-sandbox",          "sigma",  "cosandbox", "s3"),
]

SOURCES = ["ux", "s3", "lambda"]

# Bars are identified by the legend rather than by tick labels, so each needs
# its own color. One hue per family, shaded across the bars within it, so the
# families still read as groups.
FAMILY_CMAP = {
  "fslib":     "Blues",
  "getput":    "Oranges",
  "cosandbox": "Greens",
  "lambda":    "Reds",
}

def bar_colors(families):
  # A shade per bar, spread over the middle of its family's colormap: the
  # extremes are too pale to see and too dark to tell apart.
  n = {}
  for f in families:
    n[f] = n.get(f, 0) + 1
  seen = {}
  out = []
  for f in families:
    i = seen.get(f, 0)
    seen[f] = i + 1
    frac = 0.65 if n[f] == 1 else 0.45 + 0.4 * i / (n[f] - 1)
    out.append(matplotlib.colormaps[FAMILY_CMAP[f]](frac))
  return out

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
# The run's own aggregate lines, e.g.
#   mappers  inner: n 86 min 4086ms mean 10101.9ms median 8902ms p90 ... max ...
TASKSUM_RE = re.compile(r'^(mappers|reducers)\s+(inner|outer):\s+n \d+ .*?\bmean ([0-9.]+)ms')
# Corral reports only its map phase; its reduce phase is the rest of the job.
CORRAL_MAP_RE = re.compile(r'map phase: time (\d+)ms')
# Corral's per-task line, one per invocation:
#   mapper time 0 in 136 MB out 19 MB inner 753ms outer 775ms ninvoc 7 ...
# The mapper/reducer tag was added to tell the two apart (they used to be
# byte-identical); a log without it falls back to the map-phase boundary below.
CORRAL_TASK_RE = re.compile(r'\b(mapper|reducer) time \d+ in .*?\binner (\d+)ms outer (\d+)ms')
CORRAL_TASK_UNTAGGED_RE = re.compile(r'\btime \d+ in .*?\binner (\d+)ms outer (\d+)ms')

def scrape_run_stats(dname, e2e=None):
  # What a run reports beyond its end-to-end time: the phase split, the median
  # per-task gets time, and the mean per-task inner/outer times. Any key is
  # absent if the run's output doesn't carry it. Medians for gets (straggler
  # tails inflate the mean by ~30%); means for inner/outer, which is what the
  # benchmark's own aggregate lines report.
  #
  # e2e, in seconds, lets a corral run's reduce phase be derived: corral logs
  # only its map phase, and the rest of the job is the reduce phase.
  fn = bench_out(dname)
  if fn is None:
    return {}
  with open(fn, "r") as f:
    lines = f.read().split("\n")
  gets = {"mr-m": [], "mr-r": []}
  kind = None
  st = {}
  # Corral's per-task inner/outer times, gathered per task type.
  ctask = {"map": {"inner": [], "outer": []}, "reduce": {"inner": [], "outer": []}}
  # For an untagged corral log, everything logged before the map phase completes
  # is a mapper and everything after is a reducer.
  cphase = "map"
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
    if s := TASKSUM_RE.match(l):
      who = "map" if s.group(1) == "mappers" else "reduce"
      st["%s_%s_ms" % (who, s.group(2))] = int(round(float(s.group(3))))
      continue
    p = PHASE_RE.search(l)
    if p and "map_ms" not in st:
      st["map_ms"] = int(p.group(1))
      st["reduce_ms"] = int(p.group(2))
      continue
    if t := CORRAL_TASK_RE.search(l):
      who = "map" if t.group(1) == "mapper" else "reduce"
      ctask[who]["inner"].append(int(t.group(2)))
      ctask[who]["outer"].append(int(t.group(3)))
      continue
    if t := CORRAL_TASK_UNTAGGED_RE.search(l):
      ctask[cphase]["inner"].append(int(t.group(1)))
      ctask[cphase]["outer"].append(int(t.group(2)))
      continue
    if c := CORRAL_MAP_RE.search(l):
      cphase = "reduce"
      if "map_ms" not in st:
        st["map_ms"] = int(c.group(1))
        # Corral's reported job time spans the whole job (and its Lambda deploy),
        # so what is left after the map phase is the reduce phase plus that
        # deploy — see the note in the run's log about "Building Lambda function".
        if e2e is not None:
          st["reduce_ms"] = max(0, int(round(e2e * 1000)) - st["map_ms"])
  for k, name in (("mr-m", "map_gets_ms"), ("mr-r", "reduce_gets_ms")):
    if gets[k]:
      st[name] = int(statistics.median(gets[k]))
      st[name + "_n"] = len(gets[k])
  # A corral run has no aggregate lines, so its means come from its per-task
  # ones. Same statistic as the sigmaos side reports, so the columns compare.
  for who in ("map", "reduce"):
    for which in ("inner", "outer"):
      v = ctask[who][which]
      key = "%s_%s_ms" % (who, which)
      if v and key not in st:
        st[key] = int(round(statistics.mean(v)))
  return st

def fmt_ms(st, key):
  return "%6s" % (st[key] if key in st else "-")

# The keys a row reports, in print order.
ROW_KEYS = ("map_ms", "reduce_ms",
            "map_inner_ms", "map_outer_ms", "map_gets_ms",
            "reduce_inner_ms", "reduce_outer_ms", "reduce_gets_ms")

def fmt_row(name, t, st):
  return ("    %-9s e2e %6.2fs  map %s  reduce %s  |  mapper: inner %s outer %s gets %s"
          "  |  reducer: inner %s outer %s gets %s"
          % (name, t, fmt_ms(st, "map_ms"), fmt_ms(st, "reduce_ms"),
             fmt_ms(st, "map_inner_ms"), fmt_ms(st, "map_outer_ms"), fmt_ms(st, "map_gets_ms"),
             fmt_ms(st, "reduce_inner_ms"), fmt_ms(st, "reduce_outer_ms"), fmt_ms(st, "reduce_gets_ms")))

def report_runs(label, dirs, times):
  # Print what each run contributed, so that a config's mean and error bar can
  # be traced back to individual runs, and so the phase split and the per-task
  # costs are visible next to the end-to-end number they explain. inner/outer
  # are the run's own means, gets its median; all in ms.
  print("%s (%d run%s)" % (label.replace("\n", " "), len(dirs), "" if len(dirs) == 1 else "s"))
  rows = []
  for d, t in zip(dirs, times):
    st = scrape_run_stats(d, e2e=t)
    rows.append(st)
    print(fmt_row(os.path.basename(d.rstrip("/")), t, st))
  if len(rows) > 1:
    med = {}
    for k in ROW_KEYS:
      v = [ r[k] for r in rows if k in r ]
      if v:
        med[k] = int(statistics.median(v))
    print(fmt_row("median", statistics.median(times), med))

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
    # The mechanism's name is a presentation choice, so it comes from the
    # command line rather than being baked into CONFIGS: --sys-name initscript
    # relabels every co-sandbox bar without touching the keys the results
    # directories are named after.
    label = label.replace("co-sandbox", args.sys_name)
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
  # Widen with the number of bars so the tick labels — the widest of which is
  # "co-sandbox" — stay legible.
  fig, ax = plt.subplots(figsize=(max(6.4, 0.95 * nbar + 1.0), 3.2))
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
  # Spread across runs is only drawn when asked for: on a figure with this many
  # bars the caps and the ± in every label are a lot of ink, and for a
  # single-run configuration there is nothing to show.
  errs = [ e for _, _, _, e, _ in bars ] if args.error_bars else [ 0.0 ] * len(bars)
  colors = bar_colors([ f for _, f, _, _, _ in bars ])
  plt.bar(x, times, width=0.7, color=colors,
          yerr=(errs if args.error_bars else None), capsize=3,
          error_kw={"ecolor": "black", "elinewidth": 1})
  top = max(t + e for t, e in zip(times, errs))
  for i, (_, _, v, e, n) in enumerate(bars):
    txt = "%.2f±%.2f" % (v, e) if args.error_bars and n > 1 else str(round(v, 2))
    plt.text(x[i], v + (e if args.error_bars else 0) + top * 0.02, txt, ha="center", fontsize=8)
  # No x ticks: the bars are named in the legend, one entry each, in bar order.
  ax.set_xticks([])
  ax.set_ylim(bottom=0, top=top * 1.25)

  nruns = set(n for _, _, _, _, n in bars)
  if nruns != {1}:
    ylabel = "Execution Time (seconds)"
    ax.set_ylabel(ylabel)

  handles = [ matplotlib.patches.Patch(color=c, label=l)
              for c, (l, _, _, _, _) in zip(colors, bars) ]
  # Below the axes, in the space the tick labels used to occupy: inside the plot
  # a legend this long covers the bars it is naming.
  ax.legend(handles=handles, loc="upper center", bbox_to_anchor=(0.5, -0.02),
            ncol=min(4, len(handles)), fontsize=7, frameon=False)

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
  parser.add_argument("--sys-name", default="co-sandbox",
                      help="Label to use in place of 'co-sandbox' in the bar labels "
                           "(default: co-sandbox)")
  parser.add_argument("--error_bars", action="store_true", default=False,
                      help="Draw ±1 s.d. across the runs of each configuration, as error bars "
                           "and in the value labels. Off by default: the spread is always "
                           "printed per configuration on stdout regardless.")
  parser.add_argument("--app", type=str, default="wc")
  parser.add_argument("--title", type=str, default=None)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args)
