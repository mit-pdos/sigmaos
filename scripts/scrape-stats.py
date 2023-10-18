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
      return float(dstr.removesuffix(suffixes[i])) * mults[i]
  raise ValueError("Unexpected suffix for duration string {}".format(dstr))

def scrape_stats(path, regex, pos, verbose):
  p = re.compile(regex)
  with open(path, "r") as f:
    x = f.read()
  lines = [ l.strip() for l in x.split("\n") if p.match(l) ]
  if len(lines) == 0:
    raise ValueError("No matches for regex [{}]".format(regex))
  lat = [ str_dur_to_ms(l.split(" ")[pos]) for l in lines ]
  if verbose:
    for l in lat:
      print("{:.3f}ms".format(l))
      
  lat = np.array(lat)
  print("Stats for path[{}] regex[{}]:\n\tmin: {:.3f}ms\n\tmax: {:.3f}ms\n\tmean: {:.3f}ms\n\tstd: {:.3f}ms\n\tp50: {:.3f}ms\n\tp90: {:.3f}ms\n\tp99: {:.3f}ms".format(
    path,
    regex,
    min(lat),
    max(lat),
    np.mean(lat),
    np.median(lat),
    np.std(lat),
    np.percentile(lat, 90),
    np.percentile(lat, 99),
  ))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--path", type=str, required=True)
  parser.add_argument("--regex", type=str, required=True)
  parser.add_argument("--pos", type=int, default=-1)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()
  scrape_stats(path=args.path, regex=args.regex, pos=args.pos, verbose=args.v)
