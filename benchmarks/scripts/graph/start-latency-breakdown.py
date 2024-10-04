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
  fig, ax = plt.subplots(figsize=(4.8, 1.8))
#  ax.set_ylim(bottom=0)
  ax.set_ylabel("Latency (ms)")

  config   = [ "σOS" ]# "σOS-gvisor" ]
  sched    = np.array([        0.395, ])#       0.395 ])
  ns_exec  = np.array([        0.269, ])#       1.727 ])
  fs_jail  = np.array([        0.402, ])#       1.735 ])
  seccomp  = np.array([        0.811, ])#       2.200 ])
  apparmor = np.array([        0.023, ])#       0.009 ])
  bin_dl   = np.array([       11.900, ])#     169.298 ]

  x = np.arange(len(config))
  width = 0.5

#  bin_dl_plt   = plt.bar(x,    bin_dl, label="binfs paging (cold-start)")
  sched_plt    = plt.bar(x,    sched, width=width, label="σOS scheduling")
  ns_exec_plt  = plt.bar(x,  ns_exec, width=width, bottom=sched, label="Linux NS + exec")
  fs_jail_plt  = plt.bar(x,  fs_jail, width=width, bottom=sched+ns_exec, label="FS jail")
  seccomp_plt  = plt.bar(x,  seccomp, width=width, bottom=sched+ns_exec+fs_jail, label="Seccomp")
  apparmor_plt = plt.bar(x, apparmor, width=width, bottom=sched+ns_exec+fs_jail+seccomp, label="AppArmor")

#  plt.xticks(x, config)

  ax.set_xlim(right=1)
  ax.legend(loc="upper right")
  ax.tick_params(
    axis='x',          # changes apply to the x-axis
    which='both',      # both major and minor ticks are affected
    bottom=False,      # ticks along the bottom edge are off
    top=False,         # ticks along the top edge are off
    labelbottom=False) # labels along the bottom edge are off
  plt.tight_layout()
  fig.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.out)
