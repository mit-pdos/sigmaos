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
      lines = [ l.strip() for l in lines if " 90:" in l ]
      # Take latency tail, not the throughput tail.
      if len(lines) != 1:
        print(lines, "input_dir", input_dir, "x", x)
      assert(len(lines) == 1)
      line = lines[0].split(" ")
      sec = durationpy.from_str(line[-1]).total_seconds()
      msec = sec * 1000.0
      y.append(msec)
    assert(len(x_vals) == len(y))
    y_vals.append(y)
  return np.array(y_vals)

def add_data_to_graph(x, y, label, color, linestyle):
  plt.plot(x, y, label=label, color=color, linestyle=linestyle)

def finalize_graph(out):
  plt.xlabel("req/sec")
  plt.ylabel("99% Tail Latency (msec)")
  plt.title("Hotel Application Tail Latency")
  plt.legend()
  plt.savefig(out)

def graph_data(input_dir, out):
  systems=os.listdir(input_dir)
  x = get_x_axis(systems, input_dir)
  y = get_y_axes(systems, input_dir, x)
  color = "blue"
  linestyles = ["-", "--"]
  system_names = []
  for s in systems:
    if s == "Sigmaos":
      system_names.append("σOS")
    elif s == "K8s":
      system_names.append("Kubernetes")
    else:
      print("unexpected system", s)
      sys.exit(1)
  for i in range(len(systems)):
    add_data_to_graph(x, y[i], system_names[i], color, linestyles[i])
  finalize_graph(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.out)
