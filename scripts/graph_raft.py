#!/usr/bin/python3

import matplotlib.pyplot as plt
import bisect
import numpy as np
import argparse
import os
import json

def get_run_data(d_path, run):
  with open(os.path.join(d_path, run), "r") as f:
    x = f.read()
  lines = [ l for l in x.split("\n") if "In-raft" in l ]
  data = [ { "latency": int(l.split(" ")[-4]), "bytes": int(l.split(" ")[-2]) } for l in lines ]
  return data

def normalize_run(x, norm):
 return { k : x[k] / norm[k] for k in x }

def get_data(d_path, normalize):
  data = {}
  for r in os.listdir(d_path):
    if ".out" not in r:
      continue
    k = int(r.split("_")[-2])
    if k not in data.keys():
      data[k] = []
    data[k].extend(get_run_data(d_path, r))
  return data

def get_y_vals(data):
  experiments = sorted(data[1].keys())
  data = { n_replicas : [ data[n_replicas][e] for e in experiments ] for n_replicas in data.keys()  }
  return data

def graph_req_size_hist(data, out):
  n_bins = 20
  fig, ax = plt.subplots(1, 1, tight_layout=True)
  counts, bins, bars = ax.hist([ d["bytes"] for d in data[2] ], bins=n_bins)#if d["bytes"] > 500] < 1e3 ], bins=n_bins)
  print("counts", counts, "\nbins", bins)
  ax.set_title("Request size histogram")
  ax.set_xlabel("# of bytes")
  ax.set_ylabel("# of requests")
  plt.yscale("log")
  plt.savefig(out)

def graph_avg_request_latency(data, out):
  outlier_threshold = 1000
  n_bins = 20
  counts, bins = np.histogram([ d["bytes"] for d in data[2] if d["bytes"] ], n_bins)
  print("counts", counts, "\nbins", bins)
  fig, ax = plt.subplots(1, 1, tight_layout=True)
  for k in sorted(data.keys()):
    if k == 1:
      continue
    outliers = [ d["latency"] for d in data[k] if d["latency"] > outlier_threshold ]
    print("n outliers:", len(outliers))
    x = bins
    y = [ [] for b in bins ]
    for d in data[k]:
      idx = bisect.bisect_left(x, d["bytes"])
      if idx >= len(y):
        idx = idx - 1
      y[idx].append(d["latency"])
    stdev = [ np.std(i) for i in y ]
    y = [ np.mean(i) for i in y ]
    print("xy and std", x, y, stdev)
    ax.plot(x, y, label="{} replicas".format(k))
  ax.set_title("Latency (us) for request size (byte)")
  ax.set_xlabel("# of bytes")
  ax.set_ylabel("Latency (us)")
  ax.legend()
  plt.savefig(out)

def graph_absolute_request_latency(data, out):
  outlier_threshold = 1000
  n_bins = 20
  counts, bins = np.histogram([ d["bytes"] for d in data[2] if d["bytes"] ], n_bins)
  print("counts", counts, "\nbins", bins)
  fig, ax = plt.subplots(1, 1, tight_layout=True)
  for k in sorted(data.keys()):
    if k == 1:
      continue
    sorted_data = sorted(data[k], key=lambda x: x["bytes"], reverse=True)
    x = [ d["bytes"] for d in sorted_data if d["latency"] < outlier_threshold ]
    y = [ d["latency"] for d in sorted_data if d["latency"] < outlier_threshold ]
    ax.plot(x, y, label="{} replicas".format(k))
    outliers = [ d["latency"] for d in data[k] if d["latency"] > outlier_threshold ]
    print("n outliers:", len(outliers))
  ax.set_title("Latency (us) for request size (byte)")
  ax.set_xlabel("# of bytes")
  ax.set_ylabel("Latency (us)")
  ax.legend()
  plt.savefig(out)


def graph_latency_outliers(data, out):
  n_bins = 20
  counts, bins = np.histogram([ d["bytes"] for d in data[2] if d["bytes"] ], n_bins)
  print("counts", counts, "\nbins", bins)
  fig, ax = plt.subplots(1, 1, tight_layout=True)
  for k in sorted(data.keys()):
    if k == 1:
      continue
    y = [ d["latency"] for d in data[k] ]
    x = np.arange(len(y))
    ax.plot(x, y, label="{} replicas".format(k))
  ax.set_title("Latency (us) of <1KB Requests")
  ax.set_xlabel("Request #")
  ax.set_ylabel("Latency (us)")
  ax.legend()
  plt.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--rsh_out", type=str, required=True)
  parser.add_argument("--avg_rl_out", type=str, required=True)
  parser.add_argument("--abs_rl_out", type=str, required=True)
  parser.add_argument("--outliers_out", type=str, required=True)
  parser.add_argument("--normalize", action="store_true", default=False)
  parser.add_argument("--units", type=str, default="usec")
  args = parser.parse_args()
  data = get_data(args.measurement_dir, args.normalize)
  graph_req_size_hist(data, args.rsh_out)
  graph_avg_request_latency(data, args.avg_rl_out)
  graph_absolute_request_latency(data, args.abs_rl_out)
  graph_latency_outliers(data, args.outliers_out)
