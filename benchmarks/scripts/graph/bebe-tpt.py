#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.colors as colo
import numpy as np
import argparse
import os
import sys
import durationpy

def read_tpt(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  lines = [ l.strip().split("us,") for l in x.split("\n") if len(l.strip()) > 0 ]
  tpt = [ (float(l[0]), float(l[1])) for l in lines ]
  return tpt

def read_tpts(input_dir, substr1, substr2, ignore="XXXXXXXXXXXXXXXXXX"):
  fnames = [ f for f in os.listdir(input_dir) if substr1 in f and substr2 in f and ignore not in f ]
  tpts = [ read_tpt(os.path.join(input_dir, f))[1:] for f in fnames ]
  return tpts

def read_latency(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  lines = [ l.split(" ") for l in x.split("\n") if "Time" in l and "Lat" in l and "Tpt" in l ]
  # Get the time, ignoring "us"
  times = [ l[2][:-2] for l in lines ] 
  latencies = [ durationpy.from_str(l[4]) for l in lines ]
  lat = [ (float(times[i]), float(latencies[i].total_seconds() * 1000.0)) for i in range(len(times)) ]
  return lat

def read_latencies(input_dir, substr):
  fnames = [ f for f in os.listdir(input_dir) if substr in f ]
  lats = [ read_latency(os.path.join(input_dir, f)) for f in fnames ]
  if len(lats[0]) == 0:
    return []
  return lats

def get_time_range(tpts):
  start = sys.maxsize
  end = 0
  for tpt in tpts:
    if len(tpt) == 0:
      continue
    min_t = min([ t[0] for t in tpt ])
    max_t = max([ t[0] for t in tpt ])
    start = min(start, min_t)
    end = max(end, max_t)
  return (start, end)

def extend_tpts_to_range(tpts, r):
  if len(tpts) == 0:
    return
  for i in range(len(tpts)):
    last_tick = tpts[i][len(tpts[i]) - 1]
    if last_tick[i] <= r[1]:
      tpts[i].append((r[1], last_tick[1]))

# For now, only truncates after not before.
def truncate_tpts_to_range(tpts, r):
  if len(tpts) == 0:
    return
  new_tpts = []
  for i in range(len(tpts)):
    inner = []
    # Allow for the util graph to go to zero for a bit.
    runway = 10
    already_increased = False
    for j in range(len(tpts[i])):
      if tpts[i][j][1] > 1.0:
        already_increased = True
      if tpts[i][j][0] <= r[1] or tpts[i][j][1] > 0.5 or (already_increased and runway > 0):
        if already_increased and tpts[i][j][1] < 0.5:
          runway = runway - 1
        inner.append(tpts[i][j])
    new_tpts.append(inner)
  return new_tpts

def get_overall_time_range(ranges):
  start = sys.maxsize
  end = 0
  for r in ranges:
    start = min(start, r[0])
    end = max(end, r[1])
  return (start, end)

# Fit times to the data collection range, and convert us -> ms
def fit_times_to_range(tpts, time_range):
  for tpt in tpts:
    for i in range(len(tpt)):
      tpt[i] = ((tpt[i][0] - time_range[0]) / 1000.0, tpt[i][1])
  return tpts

def find_bucket(time, step_size):
  return int(time - time % step_size)

# XXX correct terminology is "window" not "bucket"
# Fit into step_size ms buckets.
def bucketize(tpts, time_range, xmin, xmax, step_size=1000):
  buckets = {}
  if xmin > -1 and xmax > -1:
    r = range(0, find_bucket(xmax - xmin, step_size) + step_size * 2, step_size)
  else:
    r = range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size)
  for i in r:
    buckets[i] = 0.0
  for tpt in tpts:
    for t in tpt:
      sub = max(0, xmin)
      if xmin != -1 and xmax != -1:
        if t[0] < xmin or t[0] > xmax:
          continue
      buckets[find_bucket(t[0] - sub, step_size)] += t[1]
  return buckets

def bucketize_latency(tpts, time_range, xmin, xmax, step_size=1000):
  buckets = {}
  if xmin > -1 and xmax > -1:
    r = range(0, find_bucket(xmax - xmin, step_size) + step_size * 2, step_size)
  else:
    r = range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size)
  for i in range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size):
    buckets[i] = []
  for tpt in tpts:
    for t in tpt:
      sub = max(0, xmin)
      if xmin != -1 and xmax != -1:
        if t[0] < xmin or t[0] > xmax:
          continue
      buckets[find_bucket(t[0] - sub, step_size)].append(t[1])
  return buckets

def buckets_to_percentile(buckets, percentile):
  for t in buckets.keys():
    if len(buckets[t]) > 0:
      buckets[t] = np.percentile(buckets[t], percentile)
    else:
      buckets[t] = 0.0
  return buckets

def buckets_to_lists(buckets):
  x = np.array(sorted(list(buckets.keys())))
  y = np.array([ buckets[x1] for x1 in x ])
  return (x, y)

def add_data_to_graph(ax, x, y, label, color, linestyle, marker):
  # Convert X indices to seconds.
  x = x / 1000.0
  return ax.plot(x, y, label=label, color=color, linestyle=linestyle, marker=marker, markevery=25, markerfacecolor=colo.to_rgba(color, 0.0), markeredgecolor=color)

def finalize_graph(fig, ax, plots, title, out, maxval):
  lns = plots[0]
  for p in plots[1:]:
    lns += p
  labels = [ l.get_label() for l in lns ]
  ax[0].legend(lns, labels, bbox_to_anchor=(.5, 1.02), loc="lower center", ncol=len(labels))
  for idx in range(len(ax)):
    ax[idx].set_xlim(left=0)
    ax[idx].set_ylim(bottom=0)
    if maxval > 0:
      ax[idx].set_xlim(right=maxval)
  # plt.legend(lns, labels)
#  plt.grid(which="minor")
#  plt.grid(which="major")
  fig.align_ylabels(ax)
  fig.savefig(out, bbox_inches="tight")

def setup_graph(nplots, units, total_ncore):
  figsize=(6.4, 4.8)
  if nplots == 1:
    figsize=(6.4, 1.2)
  if total_ncore > 0:
#    npl = nplots + 1
    npl = nplots 
  else:
    npl = nplots
  fig, tptax = plt.subplots(npl, figsize=figsize, sharex=True)
  if total_ncore > 0:
    coresax = [ tptax ]
#    tptax = tptax[:-1]
  else:
    coresax = []
  ylabels = []
  for unit in units.split(","):
    ylabel = unit
    ylabels.append(ylabel)
  plt.xlabel("Time (sec)")
#  for idx in range(len(tptax)):
#    tptax[idx].set_ylabel(ylabels[idx])
  for ax in coresax:
    ax.set_ylim((0, total_ncore + 5))
    ax.set_yticks([ nc for nc in [0, 16, 32, 48, 64, 80, 96] if nc <= total_ncore ])
    ax.set_ylabel("Cores Utilized")
  return fig, tptax, coresax

def graph_data(input_dir, title, prefix, out, nrealm, units, total_ncore, percentile, k8s, xmin, xmax):
  procd_tpts = []
  be_tpts = []
  time_ranges = []
  for i in range(nrealm):
    realm_name = "benchrealm" + str(i + 1)
    procd_tpts.append(read_tpts(input_dir, realm_name, "test-", ignore=prefix))
    be_tpts.append(read_tpts(input_dir, prefix, realm_name))
    time_ranges.append(get_time_range(be_tpts[i]))
#    time_ranges.append(get_time_range(procd_tpts[i]))
  assert(len(procd_tpts) == nrealm)
  assert(len(be_tpts) == nrealm)
  # Time range for graph
  time_range = get_overall_time_range(time_ranges)
  # Add some runway to the graph.
  time_range = (time_range[0] - 5000000.0, time_range[1])
  for i in range(len(procd_tpts)):
    extend_tpts_to_range(procd_tpts[i], time_range)
    procd_tpts[i] = truncate_tpts_to_range(procd_tpts[i], time_range)
  for i in range(nrealm):
    n_dummies = 10
    # Add some runway to the graph.
    for j in range(n_dummies):
      be_tpts[i][0].insert(0, (time_range[0] + 100.0 * j, 0.0))
    be_tpts[i] = fit_times_to_range(be_tpts[i], time_range)
  for i in range(nrealm):
    procd_tpts[i] = fit_times_to_range(procd_tpts[i], time_range)
  # Convert range ms -> sec
  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  buckets = []
  for be_tpt in be_tpts:
    buckets.append(bucketize(be_tpt, time_range, xmin, xmax, step_size=1000))
  fig, tptax, coresax = setup_graph(1, units, total_ncore)
  tptax_idx = 0
  colors = ["blue", "orange", "fuchsia", "green"]
  plots = []
  for i in range(nrealm):
    if len(be_tpts[i]) == 0:
      continue
    x, y = buckets_to_lists(buckets[i])
    if "MB" in units:
      y = y / 1000000
#    p = add_data_to_graph(tptax[tptax_idx], x, y, "Realm {}".format(i + 1), colors[i], "-", "")
#    plots.append(p)
  # If we are dealing with multiple realms...
  line_style = "solid"
  marker = ""
  for i in range(nrealm):
    x, y = buckets_to_lists(dict(procd_tpts[i][0]))
    p = add_data_to_graph(coresax[0], x, y, "Realm {}".format(i + 1), colors[i], line_style, marker)
    plots.append(p)
  ta = [ tptax ]
  ta.append(coresax[0])
  tptax = ta
  finalize_graph(fig, tptax, plots, title, out, (xmax - xmin) / 1000.0)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--prefix", type=str, required=True)
  parser.add_argument("--nrealm", type=int, default=2)
  parser.add_argument("--units", type=str, required=True)
  parser.add_argument("--total_ncore", type=int, required=True)
  parser.add_argument("--percentile", type=float, default=99.0)
  parser.add_argument("--k8s", action="store_true", default=False)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--xmin", type=int, default=-1)
  parser.add_argument("--xmax", type=int, default=-1)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.title, args.prefix, args.out, args.nrealm, args.units, args.total_ncore, args.percentile, args.k8s, args.xmin, args.xmax)
