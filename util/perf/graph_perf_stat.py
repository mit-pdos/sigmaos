#!/usr/bin/python3

import matplotlib.pyplot as plt
import numpy as np
import argparse
import os

def time_from_line(line):
  return line.split(" ")[-2]

def val_from_perf_stat(lines, stat):
  for l in lines:
    if stat in l:
      x = l.strip().split(" ")[0]
      return float(x)

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

def read_data_file(path, stat):
  with open(path, "r") as f:
    x = f.read()
  lines = x.split("\n")[:-1]
  profile = lines[0].split(" ")
  dim = int(profile[0])
  its = int(profile[1])
  n = int(profile[2])
  try:
    comp_time = float(val_from_perf_stat(lines, "seconds time elapsed")) * 1000000.0
    stat_val = float(val_from_perf_stat(lines, stat))
    return (dim, its, n, comp_time, stat_val)
  except ValueError:
    print("Invalid format:", path)
    return None

def read_data(paths, test_type, stat):
  data = {}
  for p in paths:
    if "baseline" in p or test_type not in p:
      continue
    res = read_data_file(p, stat)
    if res is None:
      continue
    dim, its, n, comp_time, stat_val = res
    if its not in data.keys():
      data[its] = []
    data[its].append((comp_time, stat_val))
  return data

def compute_tail(data, percentile):
  runtime = {}
  for k, v in data.items():
    # Get time from tuple
    t = [ x[0] for x in v ]
    runtime[k] = float(np.percentile(t, percentile))
  return runtime

def get_stat(data, tail_vals):
  mean = {}
  tail = {}
  for k, v in data.items():
    s = [ x[1] for x in v ]
    mean[k] = float(np.mean(s))
    # XXX we compute mean of the tail
    tail[k] = np.mean([ x[1] for x in v if x[0] >= tail_vals[k] ])
  return mean, tail

def get_y_stat(stat):
  y = [ stat[it] for it in sorted(stat.keys()) ]
  y = np.array(y)
  return y

def get_x(profile, stat):
  total_profile_comp_time = float(profile[3])
  profile_its = float(profile[1])
  avg_profile_comp_time = total_profile_comp_time / profile_its
  x = [ float(it) * avg_profile_comp_time / 1000.0 for it in sorted(stat.keys()) ]
  return x

def get_x_y(profile, stat):
  x = get_x(profile, stat)
  y = get_y_stat(stat)
  return (x, y)

def cutoff_at(x_y, cutoff):
  x, y = x_y
  for i in range(len(x)):
    if x[i] > cutoff:
      return x[:i], y[:i]
  return x_y

def trim(a, b):
  a_x, a_y = a
  b_x, b_y = b
  cutoff = min(max(a_x), max(b_x))
  a = cutoff_at(a, cutoff)
  b = cutoff_at(b, cutoff)
  return a, b

def plot(title, units, native_x_y, ninep_x_y, native_stat_tail_x_y=None, ninep_stat_tail_x_y=None, suffix="", percent=99):
  native_x_y, ninep_x_y = trim(native_x_y, ninep_x_y)
  if native_stat_tail_x_y is not None:
    native_stat_tail_x_y, ninep_stat_tail_x_y = trim(native_stat_tail_x_y, ninep_stat_tail_x_y)
  native_x, native_y = native_x_y
  ninep_x, ninep_y = ninep_x_y

  fig, ax = plt.subplots(1)
  ax.plot(native_x, native_y, label="Native", color="blue")
#  ax.plot(ninep_x, ninep_y, label="9p", color="orange")

  if native_stat_tail_x_y is not None:
    native_stat_tail_x, native_y = native_stat_tail_x_y
    ninep_stat_tail_x, ninep_y = ninep_stat_tail_x_y
    ax.plot(native_stat_tail_x, native_y, label="Native " + str(percent) + "%", color="blue", linestyle="dashed")
#    ax.plot(ninep_stat_tail_x, ninep_y, label="9p " + str(percent) + "%", color="orange", linestyle="dashed")

  ax.set_xlabel("Work per invocation (msec)")
  ax.set_ylabel(title + " " + units) 
  ax.legend(bbox_to_anchor=(1.05,1), loc="upper left")
  ax.set_title(title + " varying work per invocation")
  plt.savefig("perf/" + title.lower() + suffix + ".pdf", bbox_inches="tight")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--suffix", type=str, default="")
  parser.add_argument("--stat", type=str, default="context-switches")
  parser.add_argument("--percentile", type=int, default=99)
  args = parser.parse_args()
  paths = [ os.path.join(args.measurement_dir, d) for d in os.listdir(args.measurement_dir) ]
  native_profile = read_profile(paths, "native")
  print("native profile:", native_profile)
  # Read data from native run
  native_data = read_data(paths, "native", args.stat)
  native_tail = compute_tail(native_data, args.percentile)
  native_stat_mean, native_stat_tail = get_stat(native_data, native_tail)
  print(native_stat_mean)
  print(native_stat_tail)
  # Read data from 9p run
  ninep_data = read_data(paths, "9p", args.stat)
  ninep_tail = compute_tail(ninep_data, args.percentile)
  ninep_stat_mean, ninep_stat_tail = get_stat(ninep_data, ninep_tail)
  # Plot runtime
  native_stat_x_y = get_x_y(native_profile, native_stat_mean)
  ninep_stat_x_y = get_x_y(native_profile, ninep_stat_mean)
  # Plot runtime
  native_stat_tail_x_y = get_x_y(native_profile, native_stat_tail)
  ninep_stat_tail_x_y = get_x_y(native_profile, ninep_stat_tail)
  plot(args.stat, "(#)", native_stat_x_y, ninep_stat_x_y, native_stat_tail_x_y=native_stat_tail_x_y, ninep_stat_tail_x_y=ninep_stat_tail_x_y, percent=args.percentile, suffix=args.suffix)
