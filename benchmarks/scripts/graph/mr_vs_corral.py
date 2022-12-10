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
  with open(os.path.join(dname, "bench.out"), "r") as f:
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
  sfnbase = os.path.join(input_dir, "mr-wc-wiki" + datasize + ".yml")
  cfnbase = os.path.join(input_dir, "corral-" + datasize)
  sigma = [ scrape_times(sfnbase + "-cold", True), scrape_times(sfnbase + "-warm", True) ]
  corral = [ scrape_times(cfnbase + "-cold", False), scrape_times(cfnbase + "-warm", False) ]
  return (sigma, corral)

def finalize_graph(fig, ax, plots, title, out):
  plt.title(title)
  ax.legend(loc="upper left")
  fig.savefig(out)

def setup_graph():
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
  ax.set_ylabel("Execution Time (seconds)")
  return fig, ax

def graph_data(input_dir, datasize, out):
  sigma_times, corral_times = get_e2e_times(input_dir, datasize)

  fig, ax = setup_graph()

  width = 0.35
  sigmax = np.arange(2)
  corralx = [ x + width for x in sigmax ]
  sigmaplot = plt.bar(sigmax, sigma_times, width=width, label="σOS")
  corralplot = plt.bar(corralx, corral_times, width=width, label="Lambda")
  plots = [sigmaplot, corralplot]
  plt.xticks(sigmax + width / 2, ("Cold-start", "Warm-start"))

  finalize_graph(fig, ax, plots, "MapReduce WordCount Execution Time", out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--datasize", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.datasize, args.out)
