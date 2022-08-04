#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys

def read_tpt(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  lines = [ l.strip().split("us,") for l in x.split("\n") if len(l.strip()) > 0 ]
  tpt = [ (float(l[0]), float(l[1])) for l in lines ]
  return tpt

def read_tpts(input_dir, substr):
  fnames = [ f for f in os.listdir(input_dir) if substr in f ]
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

# Fit into 10ms buckets.
def bucketize(tpts, time_range):
  step_size = 10
  buckets = {}
  for i in range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size):
    buckets[i] = 0.0
  for tpt in tpts:
    for t in tpt:
      buckets[find_bucket(t[0], step_size)] += t[1]
  return buckets

def buckets_to_lists(buckets):
  x = np.array(sorted(list(buckets.keys())))
  y = np.array([ buckets[x1] for x1 in x ])
  return (x, y)

def moving_avg(y):
  # to get ms, multiply by step_size in bucketize
  window_size = 30
  moving_avgs = []
  for i in range(len(y) - window_size + 1):
    window = y[ i : i + window_size ]
    window_avg = sum(window) / window_size
    moving_avgs.append(window_avg)
  # Fill in the last few slots.
  for i in range(len(y) - len(moving_avgs)):
    window = y[len(moving_avgs):]
    window_avg = sum(window) / len(window)
    moving_avgs.append(window_avg)
  return np.array(moving_avgs)

def add_data_to_graph(ax, x, y, label, color, linestyle, normalize=True):
  if normalize:
    n = max(y)
    y = y / n
  # Convert X indices to seconds.
  x = x / 1000.0
  # normalize by max
  return ax.plot(x, y, label=label, color=color, linestyle=linestyle)

def finalize_graph(fig, ax, plots, title, out):
  plt.title(title)
  lns = plots[0]
  for p in plots[1:]:
    lns += p
  labels = [ l.get_label() for l in lns ]
  plt.legend(lns, labels)
  fig.savefig(out)

def setup_graph():
  fig, ax = plt.subplots()
  ax.set_xlabel("Time (sec)")
  ax.set_ylabel("Normalized Aggregate Throughput")
  ax2 = ax.twinx()
  ax2.set_ylabel("Cores Assigned")
  return fig, ax, ax2

def graph_data(input_dir, title, out, kv_realm, mr_realm):
  if kv_realm is None and mr_realm is None:
    procd_tpts = read_tpts(input_dir, "test")
    assert(len(procd_tpts) <= 1)
  else:
    procd_tpts = read_tpts(input_dir, kv_realm)
    print(read_tpts(input_dir, mr_realm))
    procd_tpts.append(read_tpts(input_dir, mr_realm)[0])
    assert(len(procd_tpts) == 2)
  procd_range = get_time_range(procd_tpts)
  mr_tpts = read_tpts(input_dir, "mr")
  mr_range = get_time_range(mr_tpts)
  kv_tpts = read_tpts(input_dir, "kv")
  kv_range = get_time_range(kv_tpts)
  # Time range for graph
  time_range = get_overall_time_range([procd_range, mr_range, kv_range])
  extend_tpts_to_range(procd_tpts, time_range)
  mr_tpts = fit_times_to_range(mr_tpts, time_range)
  kv_tpts = fit_times_to_range(kv_tpts, time_range)
  procd_tpts = fit_times_to_range(procd_tpts, time_range)
  # Convert range ms -> sec
  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  kv_buckets = bucketize(kv_tpts, time_range)
  fig, ax, ax2 = setup_graph()
  plots = []
  if len(kv_tpts) > 0:
    x, y = buckets_to_lists(kv_buckets)
    y = moving_avg(y)
    p = add_data_to_graph(ax, x, y, "KV Throughput", "blue", "-", normalize=True)
    plots.append(p)
  mr_buckets = bucketize(mr_tpts, time_range)
  if len(mr_tpts) > 0:
    x, y = buckets_to_lists(mr_buckets)
    y = moving_avg(y)
    p = add_data_to_graph(ax, x, y, "MR Throughput", "orange", "-", normalize=True)
    plots.append(p)
  if len(procd_tpts) > 0:
    # If we are dealing with multiple realms...
    if len(procd_tpts) > 1:
      x, y = buckets_to_lists(dict(procd_tpts[0]))
      p = add_data_to_graph(ax2, x, y, "KV Realm Cores Assigned", "blue", "--", normalize=False)
      plots.append(p)
      x, y = buckets_to_lists(dict(procd_tpts[1]))
      p = add_data_to_graph(ax2, x, y, "MR Realm Cores Assigned", "orange", "--", normalize=False)
      plots.append(p)
    else:
      x, y = buckets_to_lists(dict(procd_tpts[0]))
      p = add_data_to_graph(ax2, x, y, "Cores Assigned", "green", "--", normalize=False)
      plots.append(p)
  finalize_graph(fig, ax, plots, title, out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--kv_realm", type=str, default=None)
  parser.add_argument("--mr_realm", type=str, default=None)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.title, args.out, args.kv_realm, args.mr_realm)
