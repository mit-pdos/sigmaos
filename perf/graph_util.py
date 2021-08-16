#!/usr/bin/python3

import matplotlib.pyplot as plt
import numpy as np
import argparse
import math
import os

# Convert HH:MM:SS to seconds
def get_timestamp(l):
  return int(l.split(" ")[1][0:2]) * 3600 + int(l.split(" ")[1][-5:-3]) * 60 + int(l.split(" ")[1][-2:])

def parse_latency_file(d, fname):
  rate = int(fname.split("-")[0])
  path = os.path.join(d, fname)
  with open(path, "r") as f:
    x = f.read()
  lines = [ l for l in x.split("\n") if "elapsed" in l ]
  timestamp_and_latency = [ (get_timestamp(l), float(l.split(" ")[-2])) for l in lines ]
  # Ignore the first and last 2 seconds, during which warm-up and cool-down happens
  start = timestamp_and_latency[0][0]
  end = timestamp_and_latency[-1][0]
  latency = [ l for (t, l) in timestamp_and_latency if t >= start + 2 and t <= end - 2 ]
  return rate, latency 

def parse_latency_dir(d):
  latency = {}
  fs = os.listdir(d)
  parsed = [ parse_latency_file(d, f) for f in fs ]
  for rate, latencies in parsed:
    latency[rate] = np.array(latencies)
  return latency

def parse_util_file(d, fname, hz):
  rate = int(fname.split("-")[0])
  path = os.path.join(d, fname)
  with open(path, "r") as f:
    x = f.read()
  lines = [ l for l in x.split("\n") if len(l) > 0 ]
  util_pct = [ float(l.split(",")[0]) for l in lines ]
  for i in range(len(util_pct)):
    if math.isnan(util_pct[i]):
      util_pct[i] = 0.0
  # Ignore the first and last 2 seconds, during which warm-up and cool-down happens
  ignored = int(4.0 * hz)
  util = util_pct[ignored:-1 * ignored]
  return rate, util

def parse_util_dir(d, daemon, hz):
  util = {}
  fs = os.listdir(d)
  parsed = [ parse_util_file(d, f, hz) for f in fs if daemon in f ]
  for rate, utils in parsed:
    util[rate] = np.array(utils)
  return util

def graph_bar(data, fname, title, xlabel, ylabel, ymax=-1):
  fig, ax = plt.subplots(1)
  rates = [ rate for rate in sorted(data.keys()) ]
  means = [ np.mean(data[rate]) for rate in sorted(data.keys()) ]
  stds = [ np.std(data[rate]) for rate in sorted(data.keys()) ]
  for i in range(len(means)):
    if math.isnan(means[i]):
      print(rates[i], means[i], data[rates[i]])
  x_pos = np.arange(len(rates))
  ax.bar(x_pos, means, yerr=stds, align="center", label="9p")
  ax.set_xticks(x_pos)
  ax.set_xticklabels(rates, rotation=90)
  ax.set_xlabel(xlabel)
  ax.set_ylabel(ylabel) 
  if ymax > -1:
    ax.set_ylim(0, ymax)
  ax.legend(bbox_to_anchor=(1.05,1), loc="upper left")
  ax.set_title(title) 
  plt.savefig("perf/" + fname + ".pdf", bbox_inches="tight")

def graph_latency(latency, suffix):
  graph_bar(latency, "arrival-process-avg-latency" + suffix, "Average Latency Varying Arrival Rate", "Arrival rate (spawns per second)", "Average Latency (usec)")
  # TODO: DRY up code
  fig, ax = plt.subplots(1)
  for rate in sorted(latency.keys()):
      x = np.arange(float(len(latency[rate])))
      ax.plot(x, latency[rate], label=str(rate) + " spawns per second")
  ax.set_xlabel("Request #")
  ax.set_ylabel("Latency (usec)") 
  ax.legend(bbox_to_anchor=(1.05,1), loc="upper left")
  ax.set_title("Request Latency") 
  plt.savefig("perf/" + "arrival-process-latency" + suffix + ".pdf", bbox_inches="tight")

def graph_util(util, daemon, hz, suffix):
  graph_bar(util, "arrival-process-" + daemon + "-avg-utilization" + suffix, "Average CPU Utilization Varying Arrival Rate", "Arrival rate (spawns per second)", "CPU Utilization (%)", ymax=100)
  fig, ax = plt.subplots(1)
  for rate in sorted(util.keys()):
      x = np.arange(float(len(util[rate]))) * 1.0 / hz
      ax.plot(x, util[rate], label=str(rate) + " spawns per second")
  ax.set_xlabel("Time (usec)")
  ax.set_ylabel("CPU Utilization (%)") 
  ax.set_ylim(top=100)
  ax.legend(bbox_to_anchor=(1.05,1), loc="upper left")
  ax.set_title(daemon + " CPU utilization") 
  plt.savefig("perf/" + "arrival-process-" + daemon + "-utilization" + suffix + ".pdf", bbox_inches="tight")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--latency_dir", type=str, required=True)
  parser.add_argument("--util_dir", type=str, required=True)
  parser.add_argument("--suffix", type=str, default="")
  parser.add_argument("--util_sample_hz", type=float, default=10.0)
  parser.add_argument("--max_rate", type=int, default=-1)
  args = parser.parse_args()
  # Parsing
  latency = parse_latency_dir(args.latency_dir)
  memfsd_util = parse_util_dir(args.util_dir, "memfsd", args.util_sample_hz)
  procd_util = parse_util_dir(args.util_dir, "procd", args.util_sample_hz)
  if args.max_rate > 0:
    latency = { k : v for k, v in latency.items() if k < args.max_rate  }
    memfsd_util = { k : v for k, v in memfsd_util.items() if k < args.max_rate  }
    procd_util = { k : v for k, v in procd_util.items() if k < args.max_rate  }
  # Graphing
  graph_latency(latency, args.suffix)
  graph_util(memfsd_util, "memfsd", args.util_sample_hz, args.suffix)
  graph_util(procd_util, "procd", args.util_sample_hz, args.suffix)
