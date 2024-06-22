#!python3

import os
import argparse
import base64
import copy
import humanfriendly

def read_file(fpath):
  with open(fpath, "rb") as f:
    b = f.read()
  return str(b.hex())

def get_blocks(s, bsz, off):
  chunks = [ s[ i : i + bsz ] for i in range(off, len(s), bsz) ]
  return chunks

def count_unique_blocks(dpath, bsz_str, offset_increment, verbose):
  bsz = humanfriendly.parse_size(bsz_str, binary=True)
  print("=== Counting duplicate chunks of size {} with offset increments of {} bytes in {}:".format(humanfriendly.format_size(bsz, binary=True), offset_increment, dpath))
  fnames = sorted(os.listdir(dpath))
  for i in range(int(len(fnames)/2)):
    f1 = fnames[i]
    f2 = fnames[i + 1]
    if verbose:
      print("  {} vs {}".format(f))
    f1_contents = read_file(os.path.join(dpath, f1))
    f2_contents = read_file(os.path.join(dpath, f2))
    f1_blocks = get_blocks(f1_contents, bsz, 0)
    f1_blk_cnts = { x : 1 for x in f1_blocks }
    max_similarity = 0.0
    max_similarity_offset = 0
    max_sim_block_cnts = {}
    for off in range(0, bsz, offset_increment):
      block_cnts = copy.deepcopy(f1_blk_cnts)
      total_nblocks = len(f1_blocks)
      for b in get_blocks(f2_contents, bsz, off):
        total_nblocks += 1
        if b in block_cnts.keys():
          block_cnts[b] += 1
        else:
          block_cnts[b] = 1
      similarity = 100.0 * (1.0 - len(block_cnts) / total_nblocks)
      if verbose:
        print("Tried offset {}, got similarity {}".format(off, similarity))
      if similarity > max_similarity:
        max_similarity = similarity
        max_similarity_offset = off
        max_sim_block_cnts = copy.deepcopy(block_cnts)
    print("=== Max duplicate chunks {}/{} ({:0.2f}%) duplicate {} chunks between \"{}\" and \"{}\" at offset {}".format(total_nblocks - len(max_sim_block_cnts), total_nblocks, max_similarity, humanfriendly.format_size(bsz, binary=True), f1, f2, max_similarity_offset))
    print("    Unique chunks: {}/{}".format(len(max_sim_block_cnts), total_nblocks))
#    print("    Unique chunks: {}/{}\n{}".format(len(block_cnts), total_nblocks, sorted(block_cnts.keys())))

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--dir_path", type=str, required=True)
  parser.add_argument("--bsz", type=str, default="1KiB")
  parser.add_argument("--offset_increment", type=int, default=512)
  parser.add_argument("--verbose", action="store_true", default=False)
  args = parser.parse_args()

  count_unique_blocks(args.dir_path, args.bsz, args.offset_increment, args.verbose)
