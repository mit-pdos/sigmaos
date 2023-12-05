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
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
#  ax.set_ylim(bottom=0)
  ax.set_ylabel("Start Latency (ms)")

  config   = [ "σOS-Docker", "σOS-gvisor" ]
  sched    = np.array([        0.395,        0.395 ])
  ns_exec  = np.array([        0.269,        1.727 ])
  fs_jail  = np.array([        0.402,        1.735 ])
  seccomp  = np.array([        0.811,        2.200 ])
  apparmor = np.array([        0.023,        0.009 ])
#  bin_dl   = [      169.298,      169.298 ]

  x = np.arange(len(config))

  sched_plt    = plt.bar(x,    sched, label="σOS scheduling")
  ns_exec_plt  = plt.bar(x,  ns_exec, bottom=sched, label="Linux NS + exec")
  fs_jail_plt  = plt.bar(x,  fs_jail, bottom=sched+ns_exec, label="FS jail")
  seccomp_plt  = plt.bar(x,  seccomp, bottom=sched+ns_exec+fs_jail, label="Seccomp")
  apparmor_plt = plt.bar(x, apparmor, bottom=sched+ns_exec+fs_jail+seccomp, label="AppArmor")

  plt.xticks(x, config)

  ax.legend(loc="upper left")
  fig.savefig(out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.out)
