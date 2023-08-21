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

def read_tpts(input_dir, substr, ignore="xxxxxxxxxxxxxxxxxxxxxxxxxxxx"):
  fnames = [ f for f in os.listdir(input_dir) if substr in f and ignore not in f ]
  tpts = [ read_tpt(os.path.join(input_dir, f)) for f in fnames ]
  return tpts

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
  buckets_perc = {}
  for t in buckets.keys():
    if len(buckets[t]) > 0:
      buckets_perc[t] = np.percentile(buckets[t], percentile)
    else:
      buckets_perc[t] = 0.0
  return buckets_perc

def buckets_to_avg(buckets):
  for t in buckets.keys():
    if len(buckets[t]) > 0:
      buckets[t] = np.mean(buckets[t])
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
  #ax[0].legend(lns, labels, bbox_to_anchor=(.5, 1.02), loc="lower center", ncol=min(len(labels), 2))
  for idx in range(len(ax)):
    ax[idx].set_xlim(left=0)
    ax[idx].grid()
    if maxval > 0:
      ax[idx].set_xlim(right=maxval)
  # plt.legend(lns, labels)
  fig.align_ylabels(ax)
  fig.savefig(out, bbox_inches="tight")

def setup_graph(nplots, total_ncore):
  figsize=(6.4, 4.8)
  if nplots == 1:
    figsize=(6.4, 3.2)
  if nplots == 1:
    pcount = 1
  else:
    if total_ncore > 0:
      pcount = nplots + 1
    else:
      pcount = nplots
  fig, tptax = plt.subplots(pcount, figsize=figsize, sharex=True)
  if nplots == 1:
    coresax = []
    tptax = [ tptax ]
  else:
    if total_ncore > 0:
      coresax = [ tptax[-1] ]
      tptax = tptax[:-1]
    else:
      coresax = []
  plt.xlabel("Time (sec)")
  if nplots == 1:
    # Only put cores on the same graph for mr aggr tpt graph.
    for ax in tptax:
      coresax.append(ax)
  for ax in coresax:
    ax.grid()
    ax.set_ylim((0, total_ncore + total_ncore/4))
    ax.set_ylabel("Cores Utilized")
    ax.yaxis.set_ticks(np.arange(0, total_ncore + total_ncore/4, total_ncore/4))
  return fig, tptax, coresax

def graph_data(input_dir, title, out, hotel_realm, mr_realm, total_ncore, percentile, k8s, xmin, xmax):
  if hotel_realm is None and mr_realm is None:
    procd_tpts = read_tpts(input_dir, "test")
    assert(len(procd_tpts) <= 1)
  else:
    procd_tpts = read_tpts(input_dir, hotel_realm, ignore="mr-")
    if mr_realm != None:
      procd_tpts.append(read_tpts(input_dir, mr_realm, ignore="mr-")[0])
      assert(len(procd_tpts) == 2)
  procd_range = get_time_range(procd_tpts)
  # Time range for graph
  time_range = get_overall_time_range([procd_range])
  extend_tpts_to_range(procd_tpts, time_range)
  procd_tpts = truncate_tpts_to_range(procd_tpts, time_range)
  procd_tpts = fit_times_to_range(procd_tpts, time_range)
  # Convert range ms -> sec
  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  fig, tptax, coresax = setup_graph(1, total_ncore)
  tptax_idx = 0
  plots = []
  if len(procd_tpts) > 0:
    # If we are dealing with multiple realms...
    line_style = "solid"
    marker = "D"
    if len(procd_tpts) > 1:
      x, y = buckets_to_lists(dict(procd_tpts[0]))
      p = add_data_to_graph(coresax[0], x, y, "Image Resize LC CPU", "blue", line_style, marker)
      plots.append(p)
      x, y = buckets_to_lists(dict(procd_tpts[1]))
      p = add_data_to_graph(coresax[0], x, y, "Image Resize BE CPU", "orange", line_style, marker)
      plots.append(p)
      ta = [ ax for ax in tptax ]
      ta.append(coresax[0])
      tptax = ta
    else:
      x, y = buckets_to_lists(dict(procd_tpts[0]))
      p = add_data_to_graph(coresax[0], x, y, "Cores Utilized", "orange", line_style, marker)
      busy_idx = np.where(y >= 0.5)[0]
      print("BE Busy time", np.sum(np.diff(x)[busy_idx])/1000)

      plots.append(p)
      ta = [ ax for ax in tptax ]
      ta.append(coresax[0])
      tptax = ta
  finalize_graph(fig, tptax, plots, title, out, (xmax - xmin) / 1000.0)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--hotel_realm", type=str, default=None)
  parser.add_argument("--mr_realm", type=str, default=None)
  parser.add_argument("--total_ncore", type=int, required=True)
  parser.add_argument("--percentile", type=float, default=99.0)
  parser.add_argument("--k8s", action="store_true", default=False)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--xmin", type=int, default=-1)
  parser.add_argument("--xmax", type=int, default=-1)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.title, args.out, args.hotel_realm, args.mr_realm, args.total_ncore, args.percentile, args.k8s, args.xmin, args.xmax)
