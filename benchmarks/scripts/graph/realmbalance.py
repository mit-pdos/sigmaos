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
    min_t = min([ t[0] for t in tpt ])
    max_t = max([ t[0] for t in tpt ])
    start = min(start, min_t)
    end = max(end, max_t)
  return (start, end)

# Fit times to the data collection range, and convert us -> ms
def fit_times_to_range(tpts, time_range):
  for tpt in tpts:
    for i in range(len(tpt)):
      tpt[i] = ((tpt[i][0] - time_range[0]) / 1000.0, tpt[i][1])
  return tpts

def find_bucket(time, step_size):
  return int(time - time % step_size)

# Fit into 50ms buckets.
def bucketize(tpts, time_range):
  step_size = 100
  buckets = {}
  for i in range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size):
    buckets[i] = 0.0
  for tpt in tpts:
    for t in tpt:
      buckets[find_bucket(t[0], step_size)] += t[1]
  return buckets

def add_tpts_to_graph(tpts, label):
  x = np.array(sorted(list(tpts.keys())))
  y = np.array([ tpts[x1] for x1 in x ])
  n = max(y)
  y = y / n
  # Convert X indices to seconds.
  x = x / 1000.0
  # normalize by max
  plt.plot(x, y, label=label)

def make_graph(mr_buckets, kv_buckets, out):
  add_tpts_to_graph(mr_buckets, "MR Throughput")
  add_tpts_to_graph(kv_buckets, "KV Throughput")
  plt.xlabel("Time (sec)")
  plt.ylabel("Normalized Throughput")
  plt.title("Throughput over time")
  plt.legend()
  plt.savefig(out)

def graph_data(input_dir, out):
  # TODO: change to mr below
  mr_tpts = read_tpts(input_dir, "mr")
  mr_range = get_time_range(mr_tpts)
  kv_tpts = read_tpts(input_dir, "kv")
  kv_range = get_time_range(kv_tpts)
  # Time range for graph
  time_range = (min(kv_range[0], mr_range[0]), max(kv_range[1], mr_range[1]))
  mr_tpts = fit_times_to_range(mr_tpts, time_range)
  kv_tpts = fit_times_to_range(kv_tpts, time_range)
  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  mr_buckets = bucketize(mr_tpts, time_range)
  kv_buckets = bucketize(kv_tpts, time_range)
  make_graph(mr_buckets, kv_buckets, out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.out)
