#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def str_dur_to_ms(dstr):
  suffixes = [ "ms", "us", "µs", "ns", "s"  ]
  mults = [ 1.0, .001, .001, .000001, 1000.0 ]
  mins = 0.0
  for i in range(len(suffixes)):
    if dstr.endswith(suffixes[i]):
      if "h" in dstr and "m" in dstr and "s" in dstr:
        return None
      return float(dstr.removesuffix(suffixes[i])) * mults[i]
  raise ValueError("Unexpected suffix for duration string {}".format(dstr))

def get_lat(dpath, idx, keyword, ignorelast):
  bench_out_files = [ f for f in os.listdir(dpath) if "bench.out" in f ]
  lat = []
  for fname in bench_out_files:
    with open(os.path.join(dpath, fname)) as f:
      x = f.read()
    # Scrape for mean timing information
    all_lats = [ l.strip() for l in x.split("\n") if keyword in l and l.endswith("s") ]
    if ignorelast:
      all_lats = all_lats[:-1]
    parsed_lats = [ str_dur_to_ms(l.split(" ")[-1]) for l in all_lats ]
    lat.append(parsed_lats[idx])
  return round(np.mean(lat), 2)

def get_tpt(dpath):
  bench_out_files = [ f for f in os.listdir(dpath) if "bench.out" in f ]
  tpts = []
  for fname in bench_out_files:
    with open(os.path.join(dpath, fname)) as f:
      x = f.read()
    # Scrape for mean timing information
    all_tpts = [ float(l.strip().split(" ")[-1]) for l in x.split("\n") if "Avg req/sec server-side:" in l ]
    tpts.append(all_tpts[-1])
  return round(np.sum(tpts))

def graph_data(hotel_res_dir, socialnet_res_dir, out):
  fig, ((avg_lat, p99_lat), (p99_lat_peak, peak_tpt)) = plt.subplots(2, 2, figsize=(6.4, 2.4))

  # Scrape data
  hotel_avg_lat = get_lat(hotel_res_dir, 0, "Mean:", False)
  social_avg_lat = get_lat(socialnet_res_dir, 0, "Mean:", True)

  hotel_p99_lat = get_lat(hotel_res_dir, 0, " 99:", False)
  social_p99_lat = get_lat(socialnet_res_dir, 0, " 99: ", True)

  hotel_p99_lat_peak = get_lat(hotel_res_dir, -1, " 99:", False)
  social_p99_lat_peak = get_lat(socialnet_res_dir, -1, " 99: ", True)

  hotel_peak_tpt = get_tpt(hotel_res_dir)
  social_peak_tpt = get_tpt(socialnet_res_dir)

  # Graph data
  sys = [ "XOS-hotel", "XOS-hotel-overlay", "k8s-hotel", "XOS-socialnet", "XOS-socialnet-overlay", "k8s-socialnet", ]
  d_avg_lat      = [       hotel_avg_lat,  2.60,  4.83,      social_avg_lat,  2.75,  5.49, ]
  d_p99_lat      = [       hotel_p99_lat,  5.78, 12.76,      social_p99_lat,  6.24,  9.01, ]
  d_p99_lat_peak = [  hotel_p99_lat_peak, 66.34, 45.25, social_p99_lat_peak, 31.13, 12.86, ]
  d_peak_tpt     = [      hotel_peak_tpt, 11892,  5877,     social_peak_tpt,  3991,  1993, ]

  width = 0.25
  
  xticks = np.arange(len(sys))
  off = 0.0
  for i in range(len(sys)):
    off = width
    label = sys[i]
    avg_lat.bar([i * off], [d_avg_lat[i]], width=width, label=label)
    avg_lat.text(i * off, d_avg_lat[i] + .25, str(d_avg_lat[i]), ha="center")
    p99_lat.bar([i * off], [d_p99_lat[i]], width=width, label=label)
    p99_lat.text(i * off, d_p99_lat[i] + .25, str(d_p99_lat[i]), ha="center")
    p99_lat_peak.bar([i * off], [d_p99_lat_peak[i]], width=width, label=label)
    p99_lat_peak.text(i * off, d_p99_lat_peak[i] + .25, str(d_p99_lat_peak[i]), ha="center")
    peak_tpt.bar([i * off], [d_peak_tpt[i]], width=width, label=label)
    peak_tpt.text(i * off, d_peak_tpt[i] + .25, str(d_peak_tpt[i]), ha="center")

  avg_lat.title.set_text("Avg latency (ms), low load")
  p99_lat.title.set_text("p99 latency (ms), low load")
  p99_lat_peak.title.set_text("p99 latency (ms) at peak throughput")
  peak_tpt.title.set_text("Peak throughput (req/s)")
#  avg_lat.set_ylabel("Avg lat @ low load (ms)")
#  p99_lat.set_ylabel("99% lat @ low load (ms)")
#  p99_lat_peak.set_ylabel("99% Lat @ peak tpt (ms)")
#  peak_tpt.set_ylabel("Peak tpt (req/s)")

  for ax in [ avg_lat, p99_lat, p99_lat_peak, peak_tpt, ]:
    ax.locator_params(axis='y', nbins=4)
    ax.set_ylim(bottom=0)
    ax.tick_params(
      axis='x',          # changes apply to the x-axis
      which='both',      # both major and minor ticks are affected
      bottom=False,      # ticks along the bottom edge are off
      top=False,         # ticks along the top edge are off
      labelbottom=False) # labels along the bottom edge are off

  plt.tight_layout()
  handles, labels = peak_tpt.get_legend_handles_labels()
  fig.legend(handles, labels, ncol=len(sys) / 2, loc='lower center', bbox_to_anchor=(0.5, -0.2))
  fig.savefig(out, bbox_inches="tight")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--out", type=str, required=True)
  parser.add_argument("--hotel_res_dir", type=str, required=True)
  parser.add_argument("--socialnet_res_dir", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.hotel_res_dir, args.socialnet_res_dir, args.out)
