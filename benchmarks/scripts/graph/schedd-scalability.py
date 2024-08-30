#!/usr/bin/env python

import os
import sys
import re
import argparse
import numpy as np
import matplotlib.pyplot as plt

def str_dur_to_ms(dstr):
  suffixes = [ "ms", "us", "µs", "ns", "s"  ]
  mults = [ 1.0, .001, .001, .000001, 1000.0 ]
  for i in range(len(suffixes)):
    if dstr.endswith(suffixes[i]):
      if "h" in dstr and "m" in dstr and "s" in dstr:
        return None
      return float(dstr.removesuffix(suffixes[i])) * mults[i]
  raise ValueError("Unexpected suffix for duration string {}".format(dstr))

def scrape_file_stats(path, pat):
  with open(path, "r") as f:
    x = f.read()
  lines = [ l.strip() for l in x.split("\n") if pat.match(l) ]
  if len(lines) == 0:
    print("No matches for regex [{}] in file {}".format(regex, path))
    return []
  lat = [ str_dur_to_ms(l.split(" ")[pos]) for l in lines ]
  lat = [ l for l in lat if l is not None ]
  return lat

def tpt_stats(dpath):
  fname = [ f for f in os.listdir(dpath) if "bench.out" in f ][0]
  with open(os.path.join(dpath, fname), "r") as f:
    x = f.read()
  line = [ l.strip() for l in x.split("\n") if "Avg req/sec server-side" in l ][0]
  tpt = float(line.split(" ")[-1])
  return tpt
  
def scrape_dir_stats(measurement_dir, file_suffix, n_vm, regex, pos):
  dpath = os.path.join(measurement_dir, n_vm[1])
  pat = re.compile(regex)
  paths = [ os.path.join(dpath, "sigmaos-node-logs", f) for f in os.listdir(os.path.join(dpath, "sigmaos-node-logs")) if f.endswith(file_suffix) ]
  fstats = [ scrape_file_stats(f, pat) for f in paths ]
  # Ignore the first run, which involves booting uprocd.
  fstats = [ fstat[1:] for fstat in fstats if len(fstat) > 0 ]
  fstats_joined = []
  for fs in fstats:
    fstats_joined = fstats_joined + fs
  tpt = tpt_stats(dpath)
  return (tpt, fstats_joined)

def stats_summary(raw_stat):
  n_vm, (tpt, rst) = raw_stat
  if len(rst) > 0:
    return (n_vm, {
        "tpt": tpt,
        "min": min(rst),
        "max": max(rst),
        "avg": np.mean(rst),
        "std": np.std(rst),
        "p50": np.median(rst),
        "p90": np.percentile(rst, 90),
        "p99": np.percentile(rst, 99),
    })
  return (n_vm, {
      "tpt": 0,
      "min": 0,
      "max": 0,
      "avg": 0,
      "std": 0,
      "p50": 0,
      "p90": 0,
      "p99": 0,
  })

def graph_stats(stats_summary, out):
  x = [ n_vm[0] for (n_vm, st) in stats_summary ] 
  tpt = [ st["tpt"] for (n_vm, st) in stats_summary ]
  p50 = [ st["p50"] for (n_vm, st) in stats_summary ]
  p99 = [ st["p99"] for (n_vm, st) in stats_summary ]
  fig, ax = plt.subplots(2, figsize=(6.4, 2.4), sharex=True)
  ax[0].plot(x, tpt, label="Spawns/sec")
  ax[1].plot(x, p50, label="P50 latency")
  ax[1].plot(x, p99, label="P99 latency")
  ax[0].set_ylabel("Procs/sec")
  ax[1].set_ylabel("Start Latency (ms)")
  ax[1].set_xlabel("# of VMs in σOS deployment")
#  ax[0].set_xlim(left=1)
#  ax[1].set_xlim(left=1)
  ax[0].set_ylim(bottom=0)
  ax[1].set_ylim(bottom=0)
  ax[0].legend()
  ax[1].legend()
  plt.xticks(x)
#  plt.xlabel("Number of machines")
  fig.align_ylabels(ax)
  plt.savefig(out, bbox_inches='tight')

def print_stats_summary(stats_summary):
  for n_vm, ss in stats_summary:
    print("=== {}", n_vm[1])
    print("\t\ttpt:{}", ss["tpt"])
    print("\t\tp50:{}", ss["p50"])
    print("\t\tp99:{}", ss["p99"])

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()

  n_vms = sorted([ (int(f[:f.index("-vm")]), f) for f in os.listdir(args.measurement_dir) ], key=lambda x: (x[0], -1 * int(x[1][x[1].rindex("-"):])) )

  # Truncate beyond 4 machines
  n_vms = n_vms[:8]

  regex = ".*E2e spawn time since spawn until main"
  file_suffix = ".out"
  pos=-1

  raw_stats = [ (n_vm, scrape_dir_stats(measurement_dir=args.measurement_dir, file_suffix=file_suffix, n_vm=n_vm, regex=regex, pos=pos)) for n_vm in n_vms ]
  stats_summary = [ stats_summary(st) for st in raw_stats ]

  print_stats_summary(stats_summary)
  graph_stats(stats_summary=stats_summary, out=args.out)
