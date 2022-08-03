#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys

def get_x_axis(input_dir):
  return sorted([ int(x) for x in os.listdir(input_dir) ])

def get_y_axis(input_dir, x_vals, units):
  y_vals = []
  for x in x_vals:
    with open(os.path.join(input_dir, str(x), "bench.out")) as f:
      b = f.read()
    lines = b.split("\n")
    lines = [ l for l in lines if units in l ]
    assert(len(lines) == 1)
    line = lines[0].split(" ")
    for i in range(len(line)):
      if units in line[i]:
        y_vals.append(float(line[i - 1]))
        break
  assert(len(x_vals) == len(y_vals))
  return y_vals

def scale_y_axis(y, units):
  y = np.array(y)
  if units == "usec":
    units = "sec"
    y = y / (1000 * 1000)
  return y, units 

def add_data_to_graph(x, y, label, color, linestyle):
  plt.plot(x, y, label=label, color=color, linestyle=linestyle)

def finalize_graph(out, xlabel, ylabel, title):
  plt.xlabel(xlabel)
  plt.ylabel(ylabel)
  plt.title(title)
  plt.legend()
  plt.savefig(out)

def graph_data(input_dir, out, units, xlabel, ylabel, title):
  x = get_x_axis(input_dir)
  y = get_y_axis(input_dir, x, units)
  y, units = scale_y_axis(y, units)
  add_data_to_graph(x, y, "sigmaOS", "orange", "-")
  finalize_graph(out, xlabel, ylabel, title)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--units", type=str, required=True)
  parser.add_argument("--xlabel", type=str, required=True)
  parser.add_argument("--ylabel", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.out, args.units, args.xlabel, args.ylabel, args.title)
