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

def read_tpts(input_dir, substr1, ignore="XXXXXXXXXXXXXXXXXX"):
  fnames = [ f for f in os.listdir(input_dir) if substr1 in f and ignore not in f ]
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
  ax[0].legend(lns, labels, bbox_to_anchor=(.5, 1.02), loc="lower center", ncol=min(len(labels), 2))
  for idx in range(len(ax)):
    ax[idx].set_xlim(left=0)
    ax[idx].set_ylim(bottom=0)
    if maxval > 0:
      ax[idx].set_xlim(right=maxval)
  # plt.legend(lns, labels)
  fig.align_ylabels(ax)
  fig.savefig(out, bbox_inches="tight")

def setup_graph(nplots, units, total_ncore):
  figsize=(6.4, 4.8)
  if nplots == 1:
    figsize=(6.4, 2.4)
  np = nplots
  fig, tptax = plt.subplots(np, figsize=figsize, sharex=True)
  ylabels = []

  plt.xlabel("Time (sec)")
  tptax.set_ylabel(units)
  tptax.set_ylim((0, total_ncore + 1))
  return fig, tptax

def graph_data(input_dir, title, out, units, total_ncore, xmin, xmax):
  sys = [ "K8s", "SigmaOS" ]
  procd_tpts = []
  time_ranges = []
  for i in range(len(sys)):
    procd_tpts.append(read_tpts(os.path.join(input_dir, sys[i]), "test-", ignore="mr-"))
    time_ranges.append(get_time_range(procd_tpts[i]))
  max_tr_diff = max([ tr[1] - tr[0] for tr in time_ranges ])
  for i in range(len(procd_tpts)):
    extend_tpts_to_range(procd_tpts[i], (time_ranges[i][0], time_ranges[i][0] + max_tr_diff))
    procd_tpts[i] = truncate_tpts_to_range(procd_tpts[i], (time_ranges[i][0], time_ranges[i][0] + max_tr_diff))
  for i in range(len(sys)):
    procd_tpts[i] = fit_times_to_range(procd_tpts[i], (time_ranges[i][0], time_ranges[i][0] + max_tr_diff))
  # Convert range ms -> sec
#  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  buckets = []
  fig, tptax = setup_graph(1, units, total_ncore)
  tptax_idx = 0
  colors = ["blue", "orange", "fuchsia", "green"]
  plots = []
  # If we are dealing with multiple realms...
  line_style = "solid"
  marker = "D"
  for i in range(len(sys)):
    x, y = buckets_to_lists(dict(procd_tpts[i][0]))
    p = add_data_to_graph(tptax, x, y, "{} {}".format(sys[i], units), colors[i], line_style, marker)
    plots.append(p)
  tptax = [ tptax ]
  finalize_graph(fig, tptax, plots, title, out, (xmax - xmin) / 1000.0)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--units", type=str, required=True)
  parser.add_argument("--total_ncore", type=int, required=True)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--xmin", type=int, default=-1)
  parser.add_argument("--xmax", type=int, default=-1)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.title, args.out, args.units, args.total_ncore, args.xmin, args.xmax)
