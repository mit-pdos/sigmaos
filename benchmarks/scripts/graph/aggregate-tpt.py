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
  assert(len(tpts) == 1)
  last_tick = tpts[0][len(tpts[0]) - 1]
  if last_tick[0] <= r[1]:
    tpts[0].append((r[1], last_tick[1]))

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

# Fit into 100ms buckets.
def bucketize(tpts, time_range):
  # TODO: bucket size?
  step_size = 100
  buckets = {}
  for i in range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size):
    buckets[i] = 0.0
  for tpt in tpts:
    for t in tpt:
      buckets[find_bucket(t[0], step_size)] += t[1]
  return buckets

def add_data_to_graph(buckets, label, color, linestyle):
  x = np.array(sorted(list(buckets.keys())))
  y = np.array([ buckets[x1] for x1 in x ])
  n = max(y)
  y = y / n
  # Convert X indices to seconds.
  x = x / 1000.0
  # normalize by max
  plt.plot(x, y, label=label, color=color, linestyle=linestyle)

def finalize_graph(out):
  plt.xlabel("Time (sec)")
  plt.ylabel("Normalized Throughput")
  plt.title("Normalized throughput over time")
  plt.legend()
  plt.savefig(out)

def graph_data(input_dir, out):
  procd_tpts = read_tpts(input_dir, "test")
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
  if len(kv_tpts) > 0:
    add_data_to_graph(kv_buckets, "KV Throughput", "blue", "-")
  mr_buckets = bucketize(mr_tpts, time_range)
  if len(mr_tpts) > 0:
    add_data_to_graph(mr_buckets, "MR Throughput", "orange", "-")
  if len(procd_tpts) > 0:
    add_data_to_graph(dict(procd_tpts[0]), "Procds Assigned", "green", "--")
  finalize_graph(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.out)
