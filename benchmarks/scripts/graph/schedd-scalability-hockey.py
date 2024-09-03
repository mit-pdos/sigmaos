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

def scrape_file_stats(path, pat, regex):
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
  line = [ l.strip() for l in x.split("\n") if "Avg req/sec server-side" in l ]
  if len(line) == 0:
    print("No data for dpath {}".format(dpath))
  line = line[0]
  tpt = float(line.split(" ")[-1])
  return tpt
  
def scrape_dir_stats(measurement_dir, file_suffix, rps, regex, pos):
  dpath = os.path.join(measurement_dir, rps[1])
  pat = re.compile(regex)
  paths = [ os.path.join(dpath, "sigmaos-node-logs", f) for f in os.listdir(os.path.join(dpath, "sigmaos-node-logs")) if f.endswith(file_suffix) ]
  fstats = [ scrape_file_stats(f, pat, regex) for f in paths ]
  # Ignore the first run, which involves booting uprocd.
  fstats = [ fstat[1:] for fstat in fstats if len(fstat) > 0 ]
  fstats_joined = []
  for fs in fstats:
    fstats_joined = fstats_joined + fs
  tpt = tpt_stats(dpath)
  return (tpt, fstats_joined)

def stats_summary(raw_stat):
  rps, (tpt, rst) = raw_stat
  if len(rst) > 0:
    return (rps, {
        "tpt": tpt,
        "min": min(rst),
        "max": max(rst),
        "avg": np.mean(rst),
        "std": np.std(rst),
        "p50": np.median(rst),
        "p90": np.percentile(rst, 90),
        "p99": np.percentile(rst, 99),
    })
  return (rps, {
      "tpt": 0,
      "min": 0,
      "max": 0,
      "avg": 0,
      "std": 0,
      "p50": 0,
      "p90": 0,
      "p99": 0,
  })

def graph_stats(stats_summary, out, cutoff, server_tpt, log_scale, tpt_v_tpt):
  assert(not (tpt_v_tpt and server_tpt))
  x = [ rps[0] for (rps, st) in stats_summary ] 
  x1 = [ y for y in x ]
  tpt = [ st["tpt"] for (rps, st) in stats_summary ]
  if server_tpt:
    x = tpt
  p50 = [ st["p50"] for (rps, st) in stats_summary ]
  p90 = [ st["p90"] for (rps, st) in stats_summary ]
  p99 = [ st["p99"] for (rps, st) in stats_summary ]
  max_p99 = max(p99)
  if cutoff > 0:
    x.append(cutoff)
    x1.append(cutoff + 1000)
    p50.append(4 * max_p99)
    p99.append(4 * max_p99)
    p90.append(4 * max_p99)
  fig, ax = plt.subplots(1, figsize=(6.4, 2.4), sharex=True)
  if tpt_v_tpt:
    ax.plot(x, tpt, label="proc start rate")
  else:
    ax.plot(x, p50, label="P50 latency")
    ax.plot(x, p90, label="P90 latency")
    ax.plot(x, p99, label="P99 latency")
  if log_scale:
    ax.set_yscale("log")
  if tpt_v_tpt:
    ax.set_ylabel("Starts/sec")
  else:
    ax.set_ylabel("Start Latency (ms)")
    ax.set_ylim(top=1.1 * max_p99)
  ax.set_xlabel("Spawns/sec")
  ax.set_ylim(bottom=0)
  ax.legend()
  plt.xticks(x, rotation=45)
#  plt.xlabel("Number of machines")
  fig.align_ylabels(ax)
  plt.savefig(out, bbox_inches='tight')

def print_stats_summary(stats_summary):
  for rps, ss in stats_summary:
    print("=== {}".format(rps[1]))
    print("\t\ttpt:{:.2f}".format(ss["tpt"]))
    print("\t\tp50:{:.2f}".format(ss["p50"]))
    print("\t\tp90:{:.2f}".format(ss["p90"]))
    print("\t\tp99:{:.2f}".format(ss["p99"]))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--regex", type=str, required=True)
  parser.add_argument("--prefix", type=str, default="")
  parser.add_argument("--cutoff", type=int, default=-1)
  parser.add_argument("--server_tpt", action="store_true", default=False)
  parser.add_argument("--tpt_v_tpt", action="store_true", default=False)
  parser.add_argument("--log_scale", action="store_true", default=False)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()

  rpses = sorted([ (int(f[f.rindex("-") + 1:]), f) for f in os.listdir(args.measurement_dir) if f.startswith(args.prefix) ], key=lambda x: (x[0], -1 * int(x[1][x[1].rindex("-"):])) )
  if args.cutoff > 0:
    rpses = [ rps for rps in rpses if rps[0] < args.cutoff ]

  file_suffix = ".out"
  pos=-1

  raw_stats = [ (rps, scrape_dir_stats(measurement_dir=args.measurement_dir, file_suffix=file_suffix, rps=rps, regex=args.regex, pos=pos)) for rps in rpses ]
  stats_summary = [ stats_summary(st) for st in raw_stats ]

  print_stats_summary(stats_summary)
  graph_stats(stats_summary=stats_summary, out=args.out, cutoff=args.cutoff, server_tpt=args.server_tpt, log_scale=args.log_scale, tpt_v_tpt=args.tpt_v_tpt)
