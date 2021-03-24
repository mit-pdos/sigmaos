#!/usr/bin/python3

import matplotlib.pyplot as plt
import numpy as np
import argparse
import os

def time_from_line(line):
  return line.split(" ")[-2]

def parse_baseline_file(path):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")[:-1]
  params = lines[0].split(" ")
  dim = int(params[0])
  its = int(params[1])
  n = int(params[2])
  comp_time = int(time_from_line(lines[1]))
  setup_time = int(time_from_line(lines[-1]))
  return (dim, its, n, comp_time, setup_time)

def read_baseline(paths):
  for p in paths:
    if "baseline" in p:
      dim, iterations, n, comp_time, setup_time = parse_baseline_file(p)
      return (dim, iterations, n, comp_time, setup_time)

def read_data_file(path):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")[:-1]
  params = lines[0].split(" ")
  dim = int(params[0])
  its = int(params[1])
  n = int(params[2])
  try:
    comp_time = int(time_from_line(lines[-1]))
    return (dim, its, n, comp_time)
  except ValueError:
    print("Invalid format:", path)
    return None

def read_data(paths):
  data = {}
  for p in paths:
    res = read_data_file(p)
    if res is None:
      continue
    dim, its, n, comp_time = res
    data[its] = comp_time
  return data

def overhead(baseline, data):
  overhead = {}
  total_baseline_comp_time = float(baseline[3])
  baseline_its = float(baseline[1])
  setup_time = float(baseline[4]) # Ignore memalloc costs
  avg_baseline_comp_time = (total_baseline_comp_time - setup_time) / baseline_its
  for k, v in data.items():
    overhead[k] = (float(v) - setup_time) / (avg_baseline_comp_time * float(k))
  return overhead

def plot(baseline, overhead):
  x = sorted(overhead.keys())
  baseline_y = np.ones(len(x))
  overhead_y = [ overhead[it] for it in x ]
  print(overhead)
  plt.plot(x, baseline_y)
  plt.plot(x, overhead_y)
  plt.savefig("perf/overhead.pdf")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  args = parser.parse_args()
  paths = [ os.path.join(args.measurement_dir, d) for d in os.listdir(args.measurement_dir) ]
  baseline = read_baseline(paths)
  print("baseline:", baseline)
  data = read_data(paths)
  overhead = overhead(baseline, data)
  plot(baseline, overhead)
