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

def scrape_stats(path, inner, is_corral):
  with open(path, "r") as f:
    x = f.read()
  expected_line_contents = "MB/s"
  if is_corral:
    expected_line_contents = "ninvoc"
  lines = [ l.strip() for l in x.split("\n") if expected_line_contents in l and "mr-r-" not in l ]
  if len(lines) == 0:
    print("No matches for contents [{}] in file {}".format(expected_line_contents, path))
    return []
  tpt = [ get_tpt(l, inner, is_corral) for l in lines ]
  return tpt

def print_stats(path, tpt, verbose):
  if verbose:
    for l in tpt:
      print("{:.3f}MB/s".format(l))
      
  tpt = np.array(tpt)
  if len(tpt) == 0:
    print("!!! NO DATA !!!")
    return
  print("Stats for path[{}]:\n\tdata points: {}\n\tsum: {:.2f}MB/s\n\tmin: {:.3f}MB/s\n\tmax: {:.3f}MB/s\n\tmean: {:.3f}MB/s\n\tstd: {:.3f}MB/s\n\tp50: {:.3f}MB/s\n\tp90: {:.3f}MB/s\n\tp99: {:.3f}MB/s".format(
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

  mapper_tpts = [ scrape_stats(path=pn, inner=args.inner, is_corral=args.is_corral) for pn in paths ]
  tpt = []
  for t in mapper_tpts:
    tpt = tpt + t
  print_stats(path=args.path, tpt=tpt, verbose=args.v)
