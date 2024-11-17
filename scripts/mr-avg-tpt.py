#!/usr/bin/env python3

import os
import re
import argparse
import numpy as np

def get_tpt(s, inner, is_corral):
  if is_corral:
    if inner:
      s = s.split(" ")[-6]
    else:
      s = s.split(" ")[-8]
    tpt = s[0:len(s)-4]
  else:
    if inner:
      assert(False)
    s = s.split(" ")[-1]
    tpt = s[1:len(s)-5]
  return float(tpt)

def scrape_stats(path, inner, is_corral, mapper):
  with open(path, "r") as f:
    x = f.read()
  expected_line_contents = "MB/s"
  if is_corral:
    expected_line_contents = "ninvoc"
  if mapper:
    exclude = "mr-r-"
  else:
    exclude = "mr-m-"
  lines = x.split("\n")
  lines = [ lines[idx].strip() for idx in range(len(lines)) if expected_line_contents in lines[idx] and exclude not in lines[idx-1] ]
  if len(lines) == 0:
    print("No matches for contents [{}] in file {}".format(expected_line_contents, path))
    return []
  tpt = [ get_tpt(l, inner, is_corral) for l in lines ]
  return tpt

def get_inner_lat(line):
  return int(line[line.index("inner") + 1][:-2])

def get_outer_lat(line):
  return int(line[line.index("outer") + 1][:-2])

def get_start_latencies(path):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")
  lines = [ l.strip().split(" ") for l in lines if "inner " in l and "outer " in l ]
  lats = [ get_outer_lat(l) - get_inner_lat(l) for l in lines ]
  return lats

def print_stats(path, tpt, inner, verbose, mapper, start_lat):
  if verbose:
    for l in tpt:
      print("{:.3f}MB/s".format(l))
      
  tpt = np.array(tpt)
  if len(tpt) == 0:
    print("!!! NO DATA !!!")
    return
  if mapper:
    pfx = "Mapper"
  elif start_lat:
    pfx = "Start latency"
  else:
    pfx = "Reducer"
  if start_lat:
    print("{} stats for path[{}]:\n\tdata points: {}\n\tmin: {:.3f}ms\n\tmax: {:.3f}ms\n\tmean: {:.3f}ms\n\tmedian: {:.3f}ms".format(
      pfx,
      path,
      len(tpt),
      min(tpt),
      max(tpt),
      np.mean(tpt),
      np.median(tpt),
    ))
  else:
    t = "outer"
    if inner:
      t = "inner"
    print("{} {} stats for path[{}]:\n\tdata points: {}\n\tsum: {:.2f}MB/s\n\tmin: {:.3f}MB/s\n\tmax: {:.3f}MB/s\n\tmean: {:.3f}MB/s\n\tstd: {:.3f}MB/s\n\tp50: {:.3f}MB/s\n\tp90: {:.3f}MB/s\n\tp99: {:.3f}MB/s".format(
      pfx,
      t,
      path,
      len(tpt),
      sum(tpt),
      min(tpt),
      max(tpt),
      np.mean(tpt),
      np.std(tpt),
      np.median(tpt),
      np.percentile(tpt, 90),
      np.percentile(tpt, 99),
    ))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--path", type=str, required=True)
  parser.add_argument("--is_corral", action="store_true", default=False)
  parser.add_argument("--inner", action="store_true", default=False)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()

  paths = [ os.path.join(args.path, fn) for fn in os.listdir(args.path) if "bench.out" in fn ]

  mapper_tpts = [ scrape_stats(path=pn, inner=args.inner, is_corral=args.is_corral, mapper=True) for pn in paths ]
  tpt = []
  for t in mapper_tpts:
    tpt = tpt + t
  print_stats(path=args.path, tpt=tpt, inner=args.inner, verbose=args.v, mapper=True, start_lat=False)

  reducer_tpts = [ scrape_stats(path=pn, inner=args.inner, is_corral=args.is_corral, mapper=False) for pn in paths ]
  tpt = []
  for t in reducer_tpts:
    tpt = tpt + t
  print_stats(path=args.path, tpt=tpt, inner=args.inner, verbose=args.v, mapper=False, start_lat=False)

  start_latencies = [get_start_latencies(pn) for pn in paths][0]
  print_stats(path=args.path, tpt=start_latencies, inner=args.inner, verbose=args.v, mapper=False, start_lat=True)
