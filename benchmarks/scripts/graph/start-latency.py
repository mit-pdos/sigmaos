#!/usr/bin/env python

import matplotlib
matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def str_dur_to_ms(dstr):
  suffixes = [ "ms", "us", "µs", "ns", "s"  ]
  mults = [ 1.0, .001, .001, .000001, 1000.0 ]
  for i in range(len(suffixes)):
    if dstr.endswith(suffixes[i]):
      if "h" in dstr and "m" in dstr and "s" in dstr:
        return None
      return float(dstr.removesuffix(suffixes[i])) * mults[i]
  raise ValueError("Unexpected suffix for duration string {}".format(dstr))

def get_e2e_latency_logs(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  return [ l.strip() for l in x.split("\n") if "E2e spawn time since spawn until main" in l and "spawn-latency-" in l ]

def get_cold_start_latencies(per_node_latencies):
  lat = []
  for node_latencies in per_node_latencies:
    # First node in the cluster to run the proc incurs the cost of downloading
    # the initial proc binary from S3, which is not considered part of the
    # cold-start experiment (so ignore this node's latency). Cold-start is only
    # measured for the 2nd node to run the proc onward (they download the proc
    # from a peer in the cluster, rather than S3). This is a rough heuristic
    # check, but should be OK because we do a sanity check before returning
    if len(node_latencies) == 0 or node_latencies[0] > 100.0:
      continue
    lat.append(node_latencies[0])
  # Sanity check that we have N - 1 cold-start measurements
  assert(len(lat) == (len(per_node_latencies) - 1) or len(lat) == len(per_node_latencies) - 2)
  return lat

def get_warm_start_latencies(per_node_latencies):
  lat = []
  for node_latencies in per_node_latencies:
    # Skip the first 10 runs of the proc to avoid cold-starts
    lat = [ *lat, *node_latencies[10:] ]
  return lat

def flatten_list(l2):
  return [ x for l1 in l2 for x in l1 ]

def parse_e2e_latency_from_node_logs(lines):
  return [ str_dur_to_ms(line.split(" ")[-1]) for line in lines ]

def get_xos_cold(cold_res_dir):
  nodes = os.listdir(os.path.join(cold_res_dir, "sigmaos-node-logs"))
  per_node_latency_logs = [ get_e2e_latency_logs(os.path.join(cold_res_dir, "sigmaos-node-logs", n)) for n in nodes ]
  per_node_latencies = [ parse_e2e_latency_from_node_logs(n_logs) for n_logs in per_node_latency_logs ]
  cold_start_latencies = get_cold_start_latencies(per_node_latencies)
  return round(np.mean(cold_start_latencies), 1)

def get_xos_warm(cold_res_dir):
  nodes = os.listdir(os.path.join(cold_res_dir, "sigmaos-node-logs"))
  per_node_latency_logs = [ get_e2e_latency_logs(os.path.join(cold_res_dir, "sigmaos-node-logs", n)) for n in nodes ]
  per_node_latencies = [ parse_e2e_latency_from_node_logs(n_logs) for n_logs in per_node_latency_logs ]
  warm_start_latencies = get_warm_start_latencies(per_node_latencies)
  return round(np.mean(warm_start_latencies), 1)

def graph_data(cold_res_dir, warm_res_dir, out):
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
  ax.set_yscale("log")
  ax.set_ylabel("Start Latency (ms)")

  xos_cold = get_xos_cold(cold_res_dir)
  xos_warm = get_xos_warm(warm_res_dir)
  print("cold: {} warm: {}".format(xos_cold, xos_warm))

  sys = [     "σOS", "AWS λ", "Docker", "K8s", "Mitosis", "FAASM", ]
  cold = [ xos_cold,    1290,     2671,  1143,       3.1,     8.8, ]
  warm = [ xos_warm,      46,      469,   217,       2.8,     0.3, ]

  assert(len(sys) == len(cold))
  assert(len(sys) == len(warm))

  width = 0.35
  xticks = np.arange(len(sys))
  coldx = [ x for x in xticks ]
  warmx = [ x + width for x in xticks ]
  coldplot = plt.bar(coldx, cold, width=width, label="Cold-start")
  for i, v in enumerate(cold):
    if v > 1:
      plt.text(xticks[i], v + .25, str(v), ha="center")
    else:
      plt.text(xticks[i], v + .05, str(v), ha="center")
  warmplot = plt.bar(warmx, warm, width=width, label="Warm-start")
  for i, v in enumerate(warm):
    if v > 1:
      plt.text(xticks[i] + width, v + .25, str(v), ha="center")
    else:
      plt.text(xticks[i] + width, v + .05, str(v), ha="center")
  plt.xticks(xticks + width / 2.0, sys)

  ax.set_ylim(bottom=0, top=max(cold + warm)*2)

  ax.legend(loc="upper right")
  fig.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--cold_res_dir", type=str, required=True)
  parser.add_argument("--warm_res_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.cold_res_dir, args.warm_res_dir, args.out)
