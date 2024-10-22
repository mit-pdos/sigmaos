#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.colors as colo
import numpy as np
import argparse
import os
import sys
import durationpy

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

def read_tpt(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  lines = [ l.strip().split("us,") for l in x.split("\n") if len(l.strip()) > 0 ]
  tpt = [ (float(l[0]), float(l[1])) for l in lines ]
  return tpt

def read_tpts(input_dir, substr, ignore="xxxxxxxxxxxxxxxxxxxxxxxxxxxx"):
  fnames = [ f for f in os.listdir(input_dir) if substr in f and ignore not in f ]
  tpts = [ read_tpt(os.path.join(input_dir, f)) for f in fnames ]
  return tpts

def read_latency(fpath):
  with open(fpath, "r") as f:
    x = f.read()
  lines = [ l.split(" ") for l in x.split("\n") if "Time" in l and "Lat" in l and "Tpt" in l ]
  # Get the time, ignoring "us"
  times = [ l[2][:-2] for l in lines ] 
  latencies = [ durationpy.from_str(l[4]) for l in lines ]
  lat = [ (float(times[i]), float(latencies[i].total_seconds() * 1000.0)) for i in range(len(times)) ]
  return lat

def read_latencies(input_dir, substr):
  fnames = [ f for f in os.listdir(input_dir) if substr in f ]
  lats = [ read_latency(os.path.join(input_dir, f)) for f in fnames ]
  if len(lats[0]) == 0:
    return []
  return lats

def get_time_range(tpts):
  start = sys.maxsize
  end = 0
  for tpt in tpts:
    if len(tpt) == 0:
      continue
    min_t = min([ t[0] for t in tpt ])
    max_t = max([ t[0] for t in tpt ])
    start = min(start, min_t)
    end = max(end, max_t)
  return (start, end)

def extend_tpts_to_range(tpts, r):
  if len(tpts) == 0:
    return
  for i in range(len(tpts)):
    last_tick = tpts[i][len(tpts[i]) - 1]
    if last_tick[i] <= r[1]:
      tpts[i].append((r[1], last_tick[1]))

# For now, only truncates after not before.
def truncate_tpts_to_range(tpts, r):
  if len(tpts) == 0:
    return
  new_tpts = []
  for i in range(len(tpts)):
    inner = []
    # Allow for the util graph to go to zero for a bit.
    runway = 10
    already_increased = False
    for j in range(len(tpts[i])):
      if tpts[i][j][1] > 1.0:
        already_increased = True
      if tpts[i][j][0] <= r[1] or tpts[i][j][1] > 0.5 or (already_increased and runway > 0):
        if already_increased and tpts[i][j][1] < 0.5:
          runway = runway - 1
        inner.append(tpts[i][j])
    new_tpts.append(inner)
  return new_tpts

def get_overall_time_range(ranges):
  start = sys.maxsize
  end = 0
  for r in ranges:
    start = min(start, r[0])
    end = max(end, r[1])
  return (start, end)

# Fit times to the data collection range, and convert us -> ms
def fit_times_to_range(tpts, time_range):
  for tpt in tpts:
    for i in range(len(tpt)):
      tpt[i] = ((tpt[i][0] - time_range[0]) / 1000.0, tpt[i][1])
  return tpts

def find_bucket(time, step_size):
  return int(time - time % step_size)

# XXX correct terminology is "window" not "bucket"
# Fit into step_size ms buckets.
def bucketize(tpts, time_range, xmin, xmax, step_size=1000):
  buckets = {}
  if xmin > -1 and xmax > -1:
    r = range(0, find_bucket(xmax - xmin, step_size) + step_size * 2, step_size)
  else:
    r = range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size)
  for i in r:
    buckets[i] = 0.0
  for tpt in tpts:
    for t in tpt:
      sub = max(0, xmin)
      if xmin != -1 and xmax != -1:
        if t[0] < xmin or t[0] > xmax:
          continue
      buckets[find_bucket(t[0] - sub, step_size)] += t[1]
  return buckets

def bucketize_latency(tpts, time_range, xmin, xmax, step_size=1000):
  buckets = {}
  if xmin > -1 and xmax > -1:
    r = range(0, find_bucket(xmax - xmin, step_size) + step_size * 2, step_size)
  else:
    r = range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size)
  for i in range(0, find_bucket(time_range[1], step_size) + step_size * 2, step_size):
    buckets[i] = []
  for tpt in tpts:
    for t in tpt:
      sub = max(0, xmin)
      if xmin != -1 and xmax != -1:
        if t[0] < xmin or t[0] > xmax:
          continue
      buckets[find_bucket(t[0] - sub, step_size)].append(t[1])
  return buckets

def buckets_to_percentile(buckets, percentile):
  buckets_perc = {}
  for t in buckets.keys():
    if len(buckets[t]) > 0:
      buckets_perc[t] = np.percentile(buckets[t], percentile)
    else:
      buckets_perc[t] = 0.0
  return buckets_perc

def buckets_to_avg(buckets):
  for t in buckets.keys():
    if len(buckets[t]) > 0:
      buckets[t] = np.mean(buckets[t])
    else:
      buckets[t] = 0.0
  return buckets

def buckets_to_lists(buckets):
  x = np.array(sorted(list(buckets.keys())))
  y = np.array([ buckets[x1] for x1 in x ])
  return (x, y)

def add_data_to_graph(ax, x, y, label, color, linestyle, marker):
  # Convert X indices to seconds.
  x = x / 1000.0
  return ax.plot(x, y, label=label, color=color, linestyle=linestyle, marker=marker, markevery=25, markerfacecolor=colo.to_rgba(color, 0.0), markeredgecolor=color)

def finalize_graph(fig, ax, plots, title, out, maxval, ymax, legend_on_right):
  lns = plots[0]
  for p in plots[1:]:
    lns += p
  labels = [ l.get_label() for l in lns ]
  if legend_on_right:
    ax[0].legend(lns, labels, bbox_to_anchor=(1.02, .5), loc="center left", ncol=1)
  else:
    ax[0].legend(lns, labels, bbox_to_anchor=(.5, 1.02), loc="lower center", ncol=min(len(labels), 3))
  for idx in range(len(ax)):
    ax[idx].set_xlim(left=0)
    if idx != 0:
      ax[idx].set_ylim(bottom=0, top=ymax)
    else:
      ax[idx].set_ylim(bottom=0)
    if maxval > 0:
      ax[idx].set_xlim(right=maxval)
  # plt.legend(lns, labels)
  fig.align_ylabels(ax)
  fig.savefig(out, bbox_inches="tight")

def truncate_to_min_max(tpts, xmin, xmax):
  new_tpts = []
  if xmin > -1 and xmax > -1:
    for t in tpts:
      inner = []
      for ti in t:
        if ti[0] > xmin and ti[0] < xmax:
          inner.append((ti[0] - xmin, ti[1]))
      new_tpts.append(inner)
  else:
    new_tpts = tpts
  return new_tpts

def setup_graph(nplots, units, total_ncore):
  figsize=(6.4, 4.8)
  if nplots == 1:
    figsize=(6.4, 2.4)
  if nplots == 1:
    np = 1
  else:
    if total_ncore > 0:
      np = nplots + 1
    else:
      np = nplots
  fig, tptax = plt.subplots(np, figsize=figsize, sharex=True)
  if nplots == 1:
    coresax = []
    tptax = [ tptax ]
  else:
    if total_ncore > 0:
      coresax = [ tptax[-1] ]
      tptax = tptax[:-1]
    else:
      coresax = []
  ylabels = []
  for unit in units.split(","):
    ylabel = unit
    ylabels.append(ylabel)
  plt.xlabel("Time (sec)")
  for idx in range(len(tptax)):
    tptax[idx].set_ylabel(ylabels[idx])
    tptax[idx].locator_params(axis='y', nbins=4)
  if nplots == 1:
    # Only put cores on the same graph for BE aggr tpt graph.
    for ax in tptax:
      ax2 = ax.twinx()
      coresax.append(ax2)
  for ax in coresax:
    ax.set_ylim((0, total_ncore + 5))
    ax.set_ylabel("Cores Utilized")
    ax.set_yticks([0, 16, 32])
  return fig, tptax, coresax

def graph_data(input_dir_sigmaos, input_dir_k8s, title, out, hotel_realm, be_realm, prefix, units, total_ncore, percentile, k8s, xmin, xmax, legend_on_right):
  if hotel_realm is None and be_realm is None:
    procd_tpts = read_tpts(input_dir_sigmaos, "test")
    assert(len(procd_tpts) <= 1)
  else:
    procd_tpts = read_tpts(input_dir_sigmaos, hotel_realm, ignore=prefix)
    if be_realm != "":
      procd_tpts.append(read_tpts(input_dir_sigmaos, be_realm, ignore=prefix)[0])
      assert(len(procd_tpts) == 2)
  be_tpts = read_tpts(input_dir_sigmaos, prefix)#"mr")
  be_range = get_time_range(be_tpts)
  procd_range = get_time_range(procd_tpts)
  hotel_tpts = read_tpts(input_dir_sigmaos, "hotel")
  hotel_range = get_time_range(hotel_tpts)
  hotel_lats = read_latencies(input_dir_sigmaos, "bench.out")
  hotel_lat_range = get_time_range(hotel_lats)
  # Time range for graph
  time_range = get_overall_time_range([be_range, hotel_range, hotel_lat_range])
  # K8s measurements
  hotel_lats_k8s = read_latencies(input_dir_k8s, "bench.out")
  hotel_lat_k8s_range = get_time_range(hotel_lats_k8s)
  hotel_tpts_k8s = read_tpts(input_dir_k8s, "hotel")
  hotel_range_k8s = get_time_range(hotel_tpts_k8s)
  time_range_k8s = get_overall_time_range([hotel_range_k8s, hotel_lat_k8s_range])
  extend_tpts_to_range(procd_tpts, time_range)
  procd_tpts = truncate_tpts_to_range(procd_tpts, time_range)
  be_tpts = fit_times_to_range(be_tpts, time_range)
  hotel_tpts = fit_times_to_range(hotel_tpts, time_range)
  procd_tpts = fit_times_to_range(procd_tpts, time_range)
  hotel_lats = fit_times_to_range(hotel_lats, time_range)
  hotel_lats_k8s = fit_times_to_range(hotel_lats_k8s, time_range_k8s)
  procd_tpts = truncate_to_min_max(procd_tpts, xmin, xmax)
  # Convert range ms -> sec
  time_range = ((time_range[0] - time_range[0]) / 1000.0, (time_range[1] - time_range[0]) / 1000.0)
  hotel_buckets = bucketize(hotel_tpts, time_range, xmin, xmax, step_size=1000)
  if len(hotel_tpts) > 0 and len(be_tpts) > 0:
    fig, tptax, coresax = setup_graph(3, units, 0)#total_ncore)
  else:
    if len(hotel_lats) > 0 and len(hotel_tpts) > 0:
      fig, tptax, coresax = setup_graph(3, units, 0)#total_ncore)
    else:
      fig, tptax, coresax = setup_graph(1, units, 0)#total_ncore)
  tptax_idx = 0
  plots = []
  hotel_lat_buckets = bucketize_latency(hotel_lats, time_range, xmin, xmax, step_size=50)
  hotel_tail_lat_buckets = buckets_to_percentile(hotel_lat_buckets, percentile)
  hotel_avg_lat_buckets = buckets_to_avg(hotel_lat_buckets)
  ymax = 0
  if len(hotel_lats) > 0:
    x1, y1 = buckets_to_lists(hotel_tail_lat_buckets)
    ymax = max(ymax, max(y1))
    p_tail_lat = add_data_to_graph(tptax[tptax_idx + 1], x1, y1, "σOS-hotel " + str(int(percentile)) + "% lat", "red", "-", "")
    plots.append(p_tail_lat)
    x2, y2 = buckets_to_lists(hotel_avg_lat_buckets)
    p_avg_lat = add_data_to_graph(tptax[tptax_idx + 1], x2, y2, "σOS-hotel avg lat", "purple", "-", "")
    plots.append(p_avg_lat)
    tptax_idx = tptax_idx + 1
  hotel_lat_k8s_buckets = bucketize_latency(hotel_lats_k8s, time_range, xmin, xmax, step_size=50)
  hotel_tail_lat_k8s_buckets = buckets_to_percentile(hotel_lat_k8s_buckets, percentile)
  hotel_avg_lat_k8s_buckets = buckets_to_avg(hotel_lat_k8s_buckets)
  if len(hotel_lats_k8s) > 0:
    x1, y1 = buckets_to_lists(hotel_tail_lat_k8s_buckets)
    ymax = max(ymax, max(y1))
    p_tail_lat = add_data_to_graph(tptax[tptax_idx + 1], x1, y1, "k8s-hotel " + str(int(percentile)) + "% lat", "red", "-", "")
    plots.append(p_tail_lat)
    x2, y2 = buckets_to_lists(hotel_avg_lat_k8s_buckets)
    p_avg_lat = add_data_to_graph(tptax[tptax_idx + 1], x2, y2, "k8s-hotel avg lat", "purple", "-", "")
    plots.append(p_avg_lat)
    tptax_idx = tptax_idx + 1
  if len(hotel_tpts) > 0:
    x, y = buckets_to_lists(hotel_buckets)
    p = add_data_to_graph(tptax[0], x, y, "Client request rate", "blue", "-", "")
    plots.append(p)
    tptax_idx = tptax_idx + 1
  be_buckets = bucketize(be_tpts, time_range, xmin, xmax, step_size=1000)
  tmod=""
  if prefix == "mr-":
    tmod = "MR"
  elif prefix == "imgresize-":
    tmod = "ImgProcess"
  else:
    assert(False)
  if len(be_tpts) > 0:
    x, y = buckets_to_lists(be_buckets)
    if "MB" in units:
      y = y / 1000000
    p = add_data_to_graph(tptax[tptax_idx], x, y, "{} (BE)".format(tmod), "orange", "-", "")
    plots.append(p)
  if len(procd_tpts) > 0:
    # If we are dealing with multiple realms...
    if len(procd_tpts) > 1:
      line_style = "solid"
      marker = ""
      x, y = buckets_to_lists(dict(procd_tpts[0]))
      p = add_data_to_graph(coresax[0], x, y, "", "blue", line_style, marker)
      plots.append(p)
      x, y = buckets_to_lists(dict(procd_tpts[1]))
      p = add_data_to_graph(coresax[0], x, y, "", "orange", line_style, marker)
      plots.append(p)
      ta = [ ax for ax in tptax ]
      ta.append(coresax[0])
      tptax = ta
    else:
      x, y = buckets_to_lists(dict(procd_tpts[0]))
#      p = add_data_to_graph(coresax[0], x, y, "Cores Utilized", "green", "--", False)
#      plots.append(p)
      ta = [ ax for ax in tptax ]
#      ta.append(coresax[0])
      tptax = ta
  ymax = int(ymax * 1.1)
  finalize_graph(fig, tptax, plots, title, out, (xmax - xmin) / 1000.0, ymax, legend_on_right)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir_sigmaos", type=str, required=True)
  parser.add_argument("--measurement_dir_k8s", type=str, required=True)
  parser.add_argument("--title", type=str, required=True)
  parser.add_argument("--prefix", type=str, required=True)
  parser.add_argument("--hotel_realm", type=str, default=None)
  parser.add_argument("--be_realm", type=str, default=None)
  parser.add_argument("--units", type=str, required=True)
  parser.add_argument("--total_ncore", type=int, required=True)
  parser.add_argument("--percentile", type=float, default=99.0)
  parser.add_argument("--k8s", action="store_true", default=False)
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--xmin", type=int, default=-1)
  parser.add_argument("--xmax", type=int, default=-1)
  parser.add_argument("--legend_on_right", action="store_true", default=False)

  args = parser.parse_args()
  graph_data(args.measurement_dir_sigmaos, args.measurement_dir_k8s, args.title, args.out, args.hotel_realm, args.be_realm, args.prefix, args.units, args.total_ncore, args.percentile, args.k8s, args.xmin, args.xmax, args.legend_on_right)
