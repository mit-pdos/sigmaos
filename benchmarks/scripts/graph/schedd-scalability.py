#!python3

import os
import re
import argparse
import numpy as np

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

def scrape_dir_stats(measurement_dir, file_suffix, n_vm, regex, pos):
  dpath = os.path.join(measurement_dir, n_vm[1])
  pat = re.compile(regex)
  paths = [ os.path.join(dpath, f) for f in os.listdir(dpath) if f.endswith(file_suffix) ]
  fstats = [ scrape_file_stats(f, pat) for f in paths ]
  # Ignore the first run, which involves booting uprocd.
  fstats = [ fstat[1:] for fstat in fstats if len(fstat) > 0 ]
  fstats_joined = []
  for fs in fstats:
    fstats_joined = fstats_joined + fs
  return fstats_joined

def stats_summary(raw_stat):
  n_vm, rst = raw_stat
  if len(rst) > 0:
    return (n_vm, {
        "min": min(rst),
        "max": max(rst),
        "avg": np.mean(rst),
        "std": np.std(rst),
        "p50": np.median(rst),
        "p90": np.percentile(rst, 90),
        "p99": np.percentile(rst, 99),
    })
  return (n_vm, {
      "min": 0,
      "max": 0,
      "avg": 0,
      "std": 0,
      "p50": 0,
      "p90": 0,
      "p99": 0,
  })

def graph_stats(stats_summary):
  x = [ n_vm[0] for (n_vm, st) in stats_summary ] 
  p50 = [ st["p50"] for (n_vm, st) in stats_summary ]
  p99 = [ st["p99"] for (n_vm, st) in stats_summary ]
  plt.plot(x, p50, label="P50 Start latency")
  plt.plot(x, p99, label="P99 Start latency")
  plt.xlabel(xlabel)
  plt.ylabel(ylabel)
  plt.title(title)
  plt.legend()
  plt.savefig(out)

def print_stats_summary(stats_summary):
  for n_vm, ss in stats_summary:
    print("=== {}", n_vm[1])
    print("\t\tp50:{}", ss["p50"])
    print("\t\tp99:{}", ss["p99"])

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()

  n_vms = sorted([ (int(f[:f.index("-vm")]), f) for f in os.listdir(args.measurement_dir) ], key=lambda x: (x[0], x[1]) )

  regex = ".*E2e spawn latency until main"
  file_suffix = ".out"
  pos=-1

  raw_stats = [ (n_vm, scrape_dir_stats(measurement_dir=args.measurement_dir, file_suffix=file_suffix, n_vm=n_vm, regex=regex, pos=pos)) for n_vm in n_vms ]
  stats_summary = [ stats_summary(st) for st in raw_stats ]

  print_stats_summary(stats_summary)
  #graph_stats(stats_summary)
