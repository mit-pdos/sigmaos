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
  fig, (dial_lat, packet_lat, tpt) = plt.subplots(1, 3, figsize=(6.4, 2.4))
  # Set up lat graph

  isol = [ "XOS", "none", "Docker overlay", "K8s overlay", ]
  d_packet_lat = [ 51, 51, 97, 189 ]
  d_dial_lat = [ 304, 229, 335, 438 ]
  d_tpt = [ 9.4, 9.4, 8.58, 6.84 ]

  width = 0.15
  
  xticks = np.arange(len(isol))
  off = 0.0
  for i in range(len(isol)):
    off = width
    label = isol[i]
    packet_lat.bar([i * off], [d_packet_lat[i]], width=width, label=label)
    packet_lat.text(i * off, d_packet_lat[i] + .25, str(d_packet_lat[i]), ha="center")
    dial_lat.bar([i * off], [d_dial_lat[i]], width=width, label=label)
    dial_lat.text(i * off, d_dial_lat[i] + .25, str(d_dial_lat[i]), ha="center")
    tpt.bar([i * off], [d_tpt[i]], width=width, label=label)
    tpt.text(i * off, d_tpt[i] + .25, str(d_tpt[i]), ha="center")



#  packet_lat.set_ylim(bottom=0)
  #packet_lat.set_yscale("log")
  packet_lat.set_ylim(bottom=0, top=max(d_packet_lat)*1.1)
  packet_lat.set_ylabel("Per-packet Latency (us)")
  #dial_lat.set_yscale("log")
  dial_lat.set_ylim(bottom=0, top=max(d_dial_lat)*1.1)
  dial_lat.set_ylabel("Dial Latency (us)")
 # Set up tpt graph
  tpt.set_ylim(bottom=0, top=max(d_tpt)*1.1)
  tpt.set_ylabel("Throughput (Gb/s)")

  for ax in [ packet_lat, dial_lat, tpt ]:
    ax.tick_params(
      axis='x',          # changes apply to the x-axis
      which='both',      # both major and minor ticks are affected
      bottom=False,      # ticks along the bottom edge are off
      top=False,         # ticks along the top edge are off
      labelbottom=False) # labels along the bottom edge are off

  plt.tight_layout()
  handles, labels = tpt.get_legend_handles_labels()
  fig.legend(handles, labels, ncol=len(isol), loc='lower center', bbox_to_anchor=(0.5, -0.1))
  fig.savefig(out, bbox_inches="tight")

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.out)
