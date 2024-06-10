#!/bin/python3

import os
import argparse
import base64

def get_blocks(fpath, bsz):
  with open(fpath, "rb") as f:
    b = f.read()
  s = base64.b64encode(b)
  chunks = [ s[ i * bsz : (i+1) * bsz ] for i in range(0, len(s), bsz) ]
  return chunks

def count_unique_blocks(dpath, bsz, verbose):
  print("=== Counting unique blocks in {}:".format(dpath))
  block_cnts = {}
  total_nblocks = 0
  fnames = os.listdir(dpath)
  for f in fnames:
    if verbose:
      print("  {}".format(f))
    for b in get_blocks(os.path.join(dpath, f), bsz):
      total_nblocks += 1
      if b in block_cnts.keys():
        block_cnts[b] += 1
      else:
        block_cnts[b] = 1
  print("=== Counted {}/{} ({:0.2f}%) unique {}-KiB blocks across {} files".format(len(block_cnts), total_nblocks, 100.0 * len(block_cnts) / total_nblocks, int(bsz / 1024), len(fnames)))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--dir_path", type=str, required=True)
  parser.add_argument("--kib", type=int, default=4)
  parser.add_argument("--verbose", action="store_true", default=False)
  args = parser.parse_args()

  count_unique_blocks(args.dir_path, args.kib * 1024, args.verbose)
