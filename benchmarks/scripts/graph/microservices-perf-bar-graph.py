#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

def graph_data(out):
  fig, ((avg_lat, p99_lat), (p99_lat_peak, peak_tpt)) = plt.subplots(2, 2, figsize=(6.4, 2.4))

  sys = [ "σOS-hotel", "σOS-hotel-overlay", "k8s-hotel", "σOS-socialnet", "σOS-socialnet-overlay", "k8s-socialnet", ]
  d_avg_lat      = [  2.34,  2.60,  4.83,  2.73,  2.75,  5.49, ]
  d_p99_lat      = [  5.17,  5.78, 12.76,  5.85,  6.24,  9.01, ]
  d_p99_lat_peak = [ 31.41, 66.34, 45.25, 13.25, 31.13, 12.86, ]
  d_peak_tpt     = [ 11896, 11892,  5877,  3988,  3991,  1993, ]

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

  avg_lat.title.set_text("Avg latency, low load")
  p99_lat.title.set_text("p99 latency, low load")
  p99_lat_peak.title.set_text("p99 latency at peak throughput")
  peak_tpt.title.set_text("Peak throughput")
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

  args = parser.parse_args()
  graph_data(args.out)
