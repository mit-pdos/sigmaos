#!/usr/bin/python3

import matplotlib.pyplot as plt
import numpy as np
import argparse
import os

def get_run_data(d_path, run):
  with open(os.path.join(d_path, run), "r") as f:
    x = f.read()
  lines = [ l.split(" ") for l in x.split("\n") if "_" in l ]
  mean_latency = { l[2] : float(l[8].split(":")[1]) for l in lines }
  return mean_latency

def normalize_run(x, norm):
 return { k : x[k] / norm[k] for k in x }

def get_data(d_path, normalize):
  data = { int(r.split("_")[-2]) : get_run_data(d_path, r) for r in os.listdir(d_path) if ".txt" in r and "pprof" not in r }
  return data

def get_y_vals(data):
  experiments = sorted(data[1].keys())
  data = { n_replicas : [ data[n_replicas][e] for e in experiments ] for n_replicas in data.keys()  }
  return data


def graph_data(data, normalize, out, units):
  fig = plt.figure(figsize=(30,20))
  plt.xlabel("Microbenchmark")
  unnormalized_data = data
  if normalize:
    data = { k : normalize_run(v, data[1]) for (k, v) in data.items() }
    plt.ylabel("Slowdown vs. Unreplicated")
  else:
    plt.ylabel("Running time (" + units + ")")
  y_vals = get_y_vals(data)
  unnormalized_y_vals = get_y_vals(unnormalized_data)
  experiments = sorted(data[1].keys())
  ind = np.arange(len(experiments))
  width = 0.25
  offset = 0.0
  for n in sorted(data.keys()):
    inds = [ i + offset for i in ind ]
    bar = plt.bar(inds, y_vals[n], width, label=str(n) + " replicas")
    for i in range(len(bar)):
      height = bar[i].get_height()
      if normalize:
        label = "{:.2f} {}".format(y_vals[n][i] * unnormalized_y_vals[1][i], units)
      else:
        label = "{:.2f} {}".format(y_vals[n][i], units)
      plt.text(bar[i].get_x() + bar[i].get_width() / 2.0, height, label, ha="center", va="bottom", rotation=90)
    offset += width
  plt.xticks([ i + width for i in ind ], experiments, rotation=90)
  plt.legend()
  plt.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--normalize", action="store_true", default=False)
  parser.add_argument("--units", type=str, default="usec")
  args = parser.parse_args()
  data = get_data(args.measurement_dir, args.normalize)
  graph_data(data, args.normalize, args.out, args.units)
