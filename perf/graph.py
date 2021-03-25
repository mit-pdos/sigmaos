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
  comp_time = float(time_from_line(lines[1]))
  setup_time = float(time_from_line(lines[-1]))
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
    comp_time = float(time_from_line(lines[-1]))
    return (dim, its, n, comp_time)
  except ValueError:
    print("Invalid format:", path)
    return None

def read_data(paths):
  data = {}
  for p in paths:
    if "baseline" in p:
      continue
    res = read_data_file(p)
    if res is None:
      continue
    dim, its, n, comp_time = res
    if its not in data.keys():
      data[its] = []
    data[its].append(comp_time)
  return data

def overhead(baseline, data):
  overhead = {}
  total_baseline_comp_time = float(baseline[3])
  baseline_its = float(baseline[1])
  setup_time = float(baseline[4]) # Ignore memalloc costs
  avg_baseline_comp_time = (total_baseline_comp_time - setup_time) / baseline_its
  for k, v in data.items():
    overhead[k] = (float(np.mean(v)) - setup_time) / (avg_baseline_comp_time * float(k))
  return overhead

def runtime(baseline, data):
  runtime = {}
  total_baseline_comp_time = float(baseline[3])
  baseline_its = float(baseline[1])
  setup_time = float(baseline[4]) # Ignore memalloc costs
  avg_baseline_comp_time = (total_baseline_comp_time - setup_time) / baseline_its
  for k, v in data.items():
    runtime[k] = float(np.mean(v)) - setup_time
  return runtime

def plot_overhead(baseline, overhead):
  total_baseline_comp_time = float(baseline[3])
  baseline_its = float(baseline[1])
  setup_time = float(baseline[4]) # Ignore memalloc costs
  avg_baseline_comp_time = (total_baseline_comp_time - setup_time) / baseline_its
  x = [ float(it) * avg_baseline_comp_time / 1000.0 for it in  sorted(overhead.keys()) ]
  baseline_y = np.ones(len(x))
  overhead_y = [ overhead[it] for it in sorted(overhead.keys()) ]
  print(overhead)

  fig, ax = plt.subplots(1)
  ax.plot(x, overhead_y, label="uLambda")
  ax.plot(x, baseline_y, label="Baseline")

  ax.set_xlabel("Work per invocation (msec)")
  ax.set_ylabel("Normalized runtime")
  ax.legend()
  ax.set_title("Normalized runtime varying work per invocation")
  plt.savefig("perf/overhead.pdf")

def plot_runtime(baseline, runtime):
  total_baseline_comp_time = float(baseline[3])
  baseline_its = float(baseline[1])
  setup_time = float(baseline[4]) # Ignore memalloc costs
  avg_baseline_comp_time = (total_baseline_comp_time - setup_time) / baseline_its
  x = [ float(it) * avg_baseline_comp_time / 1000.0 for it in  sorted(runtime.keys()) ]
  baseline_y = [ float(it) * avg_baseline_comp_time for it in sorted(runtime.keys()) ]
  runtime_y = [ runtime[it] for it in sorted(runtime.keys()) ]
  runtime_y = np.array(runtime_y)
  print(runtime)

  fig, ax = plt.subplots(1)
  ax.plot(x, runtime_y, label="uLambda")
  ax.plot(x, baseline_y, label="Baseline")

  ax.set_xlabel("Work per invocation (msec)")
  ax.set_ylabel("Runtime (msec)")
  ax.legend()
  ax.set_title("Runtime varying work per invocation")
  plt.savefig("perf/runtime.pdf")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  args = parser.parse_args()
  paths = [ os.path.join(args.measurement_dir, d) for d in os.listdir(args.measurement_dir) ]
  baseline = read_baseline(paths)
  print("baseline:", baseline)
  data = read_data(paths)
  overhead = overhead(baseline, data)
  runtime = runtime(baseline, data)
  plot_overhead(baseline, overhead)
  plot_runtime(baseline, runtime)
