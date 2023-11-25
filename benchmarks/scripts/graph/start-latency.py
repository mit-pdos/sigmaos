#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def graph_data(out):
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
#  ax.set_ylim(bottom=0)
  ax.set_yscale("log")
  ax.set_ylabel("Start Latency (ms)")

  sys = [ "σOS", "σOS-ux", "Ray", "FAASM", "Mitosis", "Docker", "Kubernetes" ]
  cold = [  223,     41.5,  25.5,     8.8,       3.1,   2671.4,         1143 ]
  warm = [  1.9,      1.9,   0.6,     0.3,       3.1,    469.1,          217 ]

  assert(len(sys) == len(cold))
  assert(len(sys) == len(warm))

  width = 0.35
  xticks = np.arange(len(sys))
  coldx = [ x for x in xticks ]
  warmx = [ x + width for x in xticks ]
  coldplot = plt.bar(coldx, cold, width=width, label="Cold-start")
  for i, v in enumerate(cold):
    plt.text(xticks[i], v + .25, str(v), ha="center")
  warmplot = plt.bar(warmx, warm, width=width, label="Warm-start")
  for i, v in enumerate(warm):
    plt.text(xticks[i] + width, v + .25, str(v), ha="center")
  plt.xticks(xticks + width / 2.0, sys)

  ax.legend(loc="upper left")
  fig.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.out)
