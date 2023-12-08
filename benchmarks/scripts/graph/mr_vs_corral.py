#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def scrape_times(dname, sigma):
  with open(os.path.join(dname, "bench.out.0"), "r") as f:
    b = f.read()
  lines = b.split("\n")
  if sigma:
    lines = [ l for l in lines if "Mean:" in l ]
    t_str = lines[0].split(" ")[-1]
  else:
    lines = [ l.strip() for l in lines if "Job Execution Time:" in l ]
    t_str = lines[0].split(" ")[-1]
  t = durationpy.from_str(t_str)
  return t.total_seconds()


def get_e2e_times(input_dir, datasize):
  sfnbase = os.path.join(input_dir, "mr-wc-wiki" + datasize + "-bench.yml")
  sfns3base = os.path.join(input_dir, "mr-wc-wiki" + datasize + "-bench-s3.yml")
  cfnbase = os.path.join(input_dir, "corral-" + datasize)
  sigma = [ scrape_times(sfnbase + "-cold", True), scrape_times(sfnbase + "-warm", True) ]
  sigmas3 = [ scrape_times(sfns3base + "-cold", True), scrape_times(sfns3base + "-warm", True) ]
  corral = [ scrape_times(cfnbase + "-cold", False), scrape_times(cfnbase + "-warm", False) ]
  return (sigma, sigmas3, corral)

def finalize_graph(fig, ax, plots, title, out):
  plt.title(title)
  ax.legend(loc="lower right")
  fig.savefig(out)

def setup_graph():
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
  ax.set_ylabel("Execution Time (seconds)")
  return fig, ax

def graph_data(input_dir, datasize, out):
  sigma_times, sigmas3_times, corral_times = get_e2e_times(input_dir, datasize)

  fig, ax = setup_graph()

  width = 0.35
  sigmax = np.arange(2) * 1.5
  sigmas3x = [ x + width for x in sigmax ]
  corralx = [ x + 2 * width for x in sigmax ]
  sigmaplot = plt.bar(sigmax, sigma_times, width=width, label="σOS (UX)")
  for i, v in enumerate(sigma_times):
    plt.text(sigmax[i], v + .25, str(round(v, 2)), ha="center")
  sigmas3plot = plt.bar(sigmas3x, sigmas3_times, width=width, label="σOS (S3)")
  for i, v in enumerate(sigmas3_times):
    plt.text(sigmas3x[i], v + .25, str(round(v, 2)), ha="center")
  corralplot = plt.bar(corralx, corral_times, width=width, label="Lambda")
  for i, v in enumerate(corral_times):
    plt.text(corralx[i], v + .25, str(round(v, 2)), ha="center")
  plots = [sigmaplot, corralplot]
  plt.xticks(sigmax + width, ("Cold-start", "Warm-start"))

  finalize_graph(fig, ax, plots, "MapReduce WordCount Execution Time", out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--datasize", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.datasize, args.out)
