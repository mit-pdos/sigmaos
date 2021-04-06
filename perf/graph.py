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
  comp_time = float(time_from_line(lines[2]))
  setup_time = float(time_from_line(lines[-1]))
  return (dim, its, n, comp_time, setup_time)

def read_profile(paths, p_type):
  for p in paths:
    if "baseline" in p and p_type in p:
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

def compute_tail(data, percentile):
  runtime = {}
  for k, v in data.items():
    runtime[k] = float(np.percentile(v, percentile))
  return runtime

def get_y_runtime(runtime):
  y = [ runtime[it] for it in sorted(runtime.keys()) ]
  y = np.array(y) / 1000.0
  return y

def get_x(profile, runtime):
  total_profile_comp_time = float(profile[3])
  profile_its = float(profile[1])
  avg_profile_comp_time = total_profile_comp_time / profile_its
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

def cutoff_at(x_y, cutoff):
  x, y = x_y
  for i in range(len(x)):
    if x[i] > cutoff:
      return x[:i], y[:i]
  return x_y

def trim(a, b, c):
  a_x, a_y = a
  b_x, b_y = b
  c_x, c_y = c
  cutoff = min(max(a_x), max(b_x), max(c_x))
  a = cutoff_at(a, cutoff)
  b = cutoff_at(b, cutoff)
  c = cutoff_at(c, cutoff)
  return a, b, c

def plot(title, units, native_x_y, ninep_x_y, remote_x_y, suffix):
  native_x_y, ninep_x_y, remote_x_y = trim(native_x_y, ninep_x_y, remote_x_y)
  native_x, native_y = native_x_y
  ninep_x, ninep_y = ninep_x_y
  remote_x, remote_y = remote_x_y

  fig, ax = plt.subplots(1)
  ax.plot(native_x, native_y, label="Native (exec)")
  ax.plot(ninep_x, ninep_y, label="9p")
  ax.plot(remote_x, remote_y, label="Remote (AWS Lambda)")

  ax.set_xlabel("Work per invocation (msec)")
  ax.set_ylabel(title + " " + units) 
  ax.legend()
  ax.set_title(title + " varying work per invocation")
  plt.savefig("perf/" + title.lower() + suffix + ".pdf")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--suffix", type=str, default="")
  parser.add_argument("--percentile", type=int, default=99)
  args = parser.parse_args()
  paths = [ os.path.join(args.measurement_dir, d) for d in os.listdir(args.measurement_dir) ]
  native_profile = read_profile(paths, "native")
  remote_profile = read_profile(paths, "remote")
  print("native profile:", native_profile)
  print("remote profile:", remote_profile)
  # Read data from native run
  native_data = read_data(paths, "native")
  native_runtime = compute_mean(native_data)
  native_tail = compute_tail(native_data, args.percentile)
  # Read data from 9p run
  ninep_data = read_data(paths, "9p")
  ninep_runtime = compute_mean(ninep_data)
  ninep_tail = compute_tail(ninep_data, args.percentile)
  # Read data from remote run
  remote_data = read_data(paths, "aws")
  remote_runtime = compute_mean(remote_data)
  remote_tail = compute_tail(remote_data, args.percentile)
  # Plot runtime
  native_runtime_x_y = get_runtime_x_y(native_profile, native_runtime)
  ninep_runtime_x_y = get_runtime_x_y(native_profile, ninep_runtime)
  remote_runtime_x_y = get_runtime_x_y(remote_profile, remote_runtime)
  plot("Runtime", "(msec)", native_runtime_x_y, ninep_runtime_x_y, remote_runtime_x_y, args.suffix)
  #Plot overhead
  native_overhead_x_y = get_overhead_x_y(native_profile, native_runtime, native_runtime)
  ninep_overhead_x_y = get_overhead_x_y(native_profile, native_runtime, ninep_runtime)
  remote_overhead_x_y = get_overhead_x_y(remote_profile, native_runtime, remote_runtime)
  plot("Overhead", "", native_overhead_x_y, ninep_overhead_x_y, remote_overhead_x_y, args.suffix)
  # Plot runtime
  native_tail_x_y = get_runtime_x_y(native_profile, native_tail)
  ninep_tail_x_y = get_runtime_x_y(native_profile, ninep_tail)
  remote_tail_x_y = get_runtime_x_y(remote_profile, remote_tail)
  plot("Tail" + str(args.percentile) + "%", "(msec)", native_tail_x_y, ninep_tail_x_y, remote_tail_x_y, args.suffix)
