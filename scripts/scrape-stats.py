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

def scrape_stats(path, regex, pos):
  p = re.compile(regex)
  with open(path, "r") as f:
    x = f.read()
  lines = [ l.strip() for l in x.split("\n") if p.match(l) ]
  if len(lines) == 0:
    print("No matches for regex [{}] in file {}".format(regex, path))
    return []
  lat = [ str_dur_to_ms(l.split(" ")[pos]) for l in lines ]
  lat = [ l for l in lat if l is not None ]
  return lat

def print_stats(path, regex, lat, verbose):
  if verbose:
    for l in lat:
      print("{:.3f}ms".format(l))
      
  lat = np.array(lat)
  print("Stats for path[{}] regex[{}]:\n\tdata points: {}\n\tsum: {:.2f}ms\n\tmin: {:.3f}ms\n\tmax: {:.3f}ms\n\tmean: {:.3f}ms\n\tstd: {:.3f}ms\n\tp50: {:.3f}ms\n\tp90: {:.3f}ms\n\tp99: {:.3f}ms".format(
    path,
    regex,
    len(lat),
    sum(lat),
    min(lat),
    max(lat),
    np.mean(lat),
    np.std(lat),
    np.median(lat),
    np.percentile(lat, 90),
    np.percentile(lat, 99),
  ))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--path", type=str, required=True)
  parser.add_argument("--regex", type=str, required=True)
  parser.add_argument("--file_suffix", type=str, default=".out")
  parser.add_argument("--pos", type=int, default=-1)
  parser.add_argument("--is_dir", action="store_true", default=False)
  parser.add_argument("--v", action="store_true", default=False)
  args = parser.parse_args()

  paths = [ args.path ]
  if args.is_dir:
    paths = [ os.path.join(args.path, fn) for fn in os.listdir(args.path) if fn.endswith(args.file_suffix) ]

  lats = [ scrape_stats(path=pn, regex=args.regex, pos=args.pos) for pn in paths ]
  lat = []
  for l in lats:
    lat = lat + l
  print_stats(path=args.path, regex=args.regex, lat=lat, verbose=args.v)
