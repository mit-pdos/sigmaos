#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import glob
import os
import sys
import durationpy

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

# One entry per configuration the graph can show. Each becomes an optional
# --<key>_dir argument holding that run's results directory; only the ones
# supplied on the command line are plotted, in this order. "kind" selects how
# the run's execution time is scraped, and "family" only picks the bar color, so
# that the fslib/getput/cosandbox/lambda groups are distinguishable at a glance.
CONFIGS = [
  ("ux",                   "UX\nfslib",       "sigma",  "fslib"),
  ("ux_getput_mapper",     "UX\ngetput\nmap", "sigma",  "getput"),
  ("ux_getput_reducer",    "UX\ngetput\nred", "sigma",  "getput"),
  ("ux_getput_both",       "UX\ngetput\nboth", "sigma", "getput"),
  ("ux_cosandbox_mapper",  "UX\ncosbx\nmap",  "sigma",  "cosandbox"),
  ("ux_cosandbox_reducer", "UX\ncosbx\nred",  "sigma",  "cosandbox"),
  ("ux_cosandbox_both",    "UX\ncosbx\nboth", "sigma",  "cosandbox"),
  ("s3",                   "S3\nfslib",       "sigma",  "fslib"),
  ("s3_getput_mapper",     "S3\ngetput\nmap", "sigma",  "getput"),
  ("s3_getput_reducer",    "S3\ngetput\nred", "sigma",  "getput"),
  ("s3_getput_both",       "S3\ngetput\nboth", "sigma", "getput"),
  ("s3_cosandbox_mapper",  "S3\ncosbx\nmap",  "sigma",  "cosandbox"),
  ("s3_cosandbox_reducer", "S3\ncosbx\nred",  "sigma",  "cosandbox"),
  ("s3_cosandbox_both",    "S3\ncosbx\nboth", "sigma",  "cosandbox"),
  ("corral",               "λ-mr",            "corral", "lambda"),
]

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

def scrape_times(dname, sigma):
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

def collect(args):
  # The (label, family, seconds) of every configuration which was both supplied
  # and has a scrapable result, in CONFIGS order.
  bars = []
  for key, label, kind, family in CONFIGS:
    dname = getattr(args, key + "_dir")
    if dname is None:
      continue
    t = scrape_times(dname, kind == "sigma")
    if t is None:
      print("Warning: no benchmark output in %s; skipping %s" % (dname, key), file=sys.stderr)
      continue
    bars.append((label, family, t))
  return bars

def setup_graph(nbar):
  # Widen with the number of bars so the tick labels stay legible.
  fig, ax = plt.subplots(figsize=(max(6.4, 0.85 * nbar + 1.0), 3.2))
  ax.set_ylabel("Execution Time (seconds)")
  return fig, ax

def graph_data(args):
  bars = collect(args)
  if len(bars) == 0:
    print("Error: no configurations supplied (pass at least one --<config>_dir)", file=sys.stderr)
    sys.exit(1)

  fig, ax = setup_graph(len(bars))

  x = np.arange(len(bars))
  times = [ t for _, _, t in bars ]
  colors = [ FAMILY_COLORS[f] for _, f, _ in bars ]
  plt.bar(x, times, width=0.7, color=colors)
  for i, v in enumerate(times):
    plt.text(x[i], v + max(times) * 0.02, str(round(v, 2)), ha="center", fontsize=8)
  ax.set_xticks(x)
  ax.set_xticklabels([ l for l, _, _ in bars ], fontsize=7)
  ax.set_ylim(bottom=0, top=max(times) * 1.25)
  ax.tick_params(axis='x', bottom=False)

  # One legend entry per family which actually appears.
  families = []
  for _, f, _ in bars:
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
  for key, label, _, _ in CONFIGS:
    parser.add_argument("--" + key + "_dir", type=str, default=None,
                        help="Results directory for the %s configuration" % label.replace("\n", " "))
  parser.add_argument("--app", type=str, default="wc")
  parser.add_argument("--title", type=str, default=None)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args)
