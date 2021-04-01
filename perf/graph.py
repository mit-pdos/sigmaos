#!/usr/bin/python3

import matplotlib.pyplot as plt
import numpy as np
import argparse
import os

def time_from_line(line):
  return line.split(" ")[-2]

def parse_profile_file(path):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")[:-1]
  profile = lines[0].split(" ")
  dim = int(profile[0])
  its = int(profile[1])
  n = int(profile[2])
  comp_time = float(time_from_line(lines[1]))
  setup_time = float(time_from_line(lines[-1]))
  return (dim, its, n, comp_time, setup_time)

def read_profile(paths):
  for p in paths:
    if "baseline" in p:
      dim, iterations, n, comp_time, setup_time = parse_profile_file(p)
      return (dim, iterations, n, comp_time, setup_time)

def read_data_file(path):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")[:-1]
  profile = lines[0].split(" ")
  dim = int(profile[0])
  its = int(profile[1])
  n = int(profile[2])
  try:
    comp_time = float(time_from_line(lines[-1]))
    return (dim, its, n, comp_time)
  except ValueError:
    print("Invalid format:", path)
    return None

def read_data(paths, test_type):
  data = {}
  for p in paths:
    if "baseline" in p or test_type not in p:
      continue
    res = read_data_file(p)
    if res is None:
      continue
    dim, its, n, comp_time = res
    if its not in data.keys():
      data[its] = []
    data[its].append(comp_time)
  return data

def compute_mean(data):
  runtime = {}
  for k, v in data.items():
    runtime[k] = float(np.mean(v))
  return runtime

def get_y_runtime(runtime):
  y = [ runtime[it] for it in sorted(runtime.keys()) ]
  y = np.array(y) / 1000.0
  return y

def get_x(profile, runtime):
  total_profile_comp_time = float(profile[3])
  profile_its = float(profile[1])
  setup_time = float(profile[4]) # Ignore memalloc costs
  avg_profile_comp_time = (total_profile_comp_time - setup_time) / profile_its
  x = [ float(it) * avg_profile_comp_time / 1000.0 for it in sorted(runtime.keys()) ]
  return x

def get_runtime_x_y(profile, runtime):
  x = get_x(profile, runtime)
  y = get_y_runtime(runtime)
  return (x, y)

def get_overhead_x_y(profile, baseline, runtime):
  x = get_x(profile, runtime)
  y = [ runtime[it] / baseline[it] for it in sorted(runtime.keys()) ]
  y = np.array(y)
  return (x, y)

def plot(title, units, native_x_y, ninep_x_y):
  native_x, native_y = native_x_y
  ninep_x, ninep_y = ninep_x_y

  fig, ax = plt.subplots(1)
  ax.plot(native_x, native_y, label="Native (exec)")
  ax.plot(ninep_x, ninep_y, label="uLambda")

  ax.set_xlabel("Work per invocation (msec)")
  ax.set_ylabel(title + " " + units) 
  ax.legend()
  ax.set_title(title + " varying work per invocation")
  plt.savefig("perf/" + title.lower() + ".pdf")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  args = parser.parse_args()
  paths = [ os.path.join(args.measurement_dir, d) for d in os.listdir(args.measurement_dir) ]
  profile = read_profile(paths)
  print("profile:", profile)
  # Read data from native run
  native_data = read_data(paths, "native")
  native_runtime = compute_mean(native_data)
  # Read data from 9p run
  ninep_data = read_data(paths, "9p")
  ninep_runtime = compute_mean(ninep_data)
  # Plot runtime
  native_runtime_x_y = get_runtime_x_y(profile, native_runtime)
  ninep_runtime_x_y = get_runtime_x_y(profile, ninep_runtime)
  plot("Runtime", "(msec)", native_runtime_x_y, ninep_runtime_x_y)
  #Plot overhead
  native_overhead_x_y = get_overhead_x_y(profile, native_runtime, native_runtime)
  ninep_overhead_x_y = get_overhead_x_y(profile, native_runtime, ninep_runtime)
  plot("Overhead", "", native_overhead_x_y, ninep_overhead_x_y)
