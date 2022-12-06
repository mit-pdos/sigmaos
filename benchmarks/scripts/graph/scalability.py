#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def get_x_axis(systems, input_dir):
  return sorted([ int(x) for x in os.listdir(os.path.join(input_dir, systems[0])) ])

def get_y_axes(systems, input_dir, x_vals):
  y_vals = []
  systems=os.listdir(input_dir)
  for system in systems:
    y = []
    for x in x_vals:
      with open(os.path.join(input_dir, system, str(x), "bench.out")) as f:
        b = f.read()
      lines = b.split("\n")
      lines = [ l for l in lines if "Mean:" in l ]
      # Take latency mean, not throughput mean.
      if len(lines) != 2:
        print(lines, "input_dir", input_dir, "x", x)
      assert(len(lines) == 2)
      line = lines[0].split(" ")
      y.append(durationpy.from_str(line[-1]))
    assert(len(x_vals) == len(y))
    y_vals.append(y)
  return np.array(y_vals)

def add_data_to_graph(x, y, label, color, linestyle):
  plt.plot(x, y, label=label, color=color, linestyle=linestyle)

def finalize_graph(out, xlabel, ylabel, title):
  plt.xlabel(xlabel)
  plt.ylabel(ylabel)
  plt.title(title)
  plt.legend()
  plt.savefig(out)

def graph_data(input_dir, out, xlabel, ylabel, title, speedup):
  systems=os.listdir(input_dir)
  x = get_x_axis(systems, input_dir)
  y = get_y_axes(systems, input_dir, x)
  for i in range(len(y)):
    if speedup:
      y[i] = y[i][0] / y[i]
  color = "orange"
  if "kv" in input_dir:
    color = "blue"
  linestyles = ["-", "--"]
  for i in range(len(systems)):
    add_data_to_graph(x, y[i], systems[i], color, linestyles[i])
  finalize_graph(out, xlabel, ylabel, title)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--xlabel", type=str, required=True)
  parser.add_argument("--ylabel", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--speedup", default=False, action="store_true")
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.out, args.xlabel, args.ylabel, args.title, args.speedup)
