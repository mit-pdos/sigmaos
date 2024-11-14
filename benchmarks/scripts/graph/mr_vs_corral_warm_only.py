#!/usr/bin/env python

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os
import sys
import durationpy

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

def scrape_times(dname, sigma):
  with open(os.path.join(dname, "bench.out.0"), "r") as f:
    b = f.read()
  lines = b.split("\n")
  if sigma:
    lines = [ l for l in lines if "Mean:" in l ]
    t_str = lines[0].split(" ")[-1]
  else:
    lines = [ l.strip() for l in lines if "Job Execution Time:" in l ]
    t_str = lines[0].split(" ")[-1]
  t = durationpy.from_str(t_str)
  return t.total_seconds()


def get_e2e_times(input_dir, app, datasize, granular, noux):
  gr = ""
  if granular:
    gr = "-granular"
  sfns3base = os.path.join(input_dir, "mr-" + app + "-wiki" + datasize + gr + "-bench-s3.yml")
  cfnbase = os.path.join(input_dir, "corral-" + app + "-wiki" + datasize + gr)
  sigmas3 = [ scrape_times(sfns3base + "-warm", True) ]
  corral = [ scrape_times(cfnbase + "-warm", False) ]
  if not noux:
    sfnbase = os.path.join(input_dir, "mr-" + app + "-wiki" + datasize + gr + "-bench.yml")
    sigmaux = [ scrape_times(sfnbase + "-warm", True) ]
  else:
    sigmaux = sigmas3
  return (sigmaux, sigmas3, corral)

def finalize_graph(fig, ax, plots, title, out):
  plt.title(title)
  ax.legend(loc="lower right")
  fig.savefig(out)

def setup_graph():
  fig, ax = plt.subplots(figsize=(6.4, 2.4))
  ax.set_ylabel("Execution Time (seconds)")
  return fig, ax

def graph_data(input_dir, app, datasize, granular, noux, out):
  sigma_times, sigmas3_times, corral_times = get_e2e_times(input_dir, app, datasize, granular, noux)

  fig, ax = setup_graph()

  width = 0.35
  sigmax = np.arange(1) * 1.5
  sigmas3x = [ x + width for x in sigmax ]
  corralx = [ x + 2 * width for x in sigmax ]
  if not noux:
    sigmaplot = plt.bar(sigmax, sigma_times, width=width, label="σOS-mr (UX)")
    for i, v in enumerate(sigma_times):
      plt.text(sigmax[i], v + .25, str(round(v, 2)), ha="center")
  sigmas3plot = plt.bar(sigmas3x, sigmas3_times, width=width, label="σOS-mr")
  for i, v in enumerate(sigmas3_times):
    plt.text(sigmas3x[i], v + .25, str(round(v, 2)), ha="center")
  corralplot = plt.bar(corralx, corral_times, width=width, label="λ-mr")
  for i, v in enumerate(corral_times):
    plt.text(corralx[i], v + .25, str(round(v, 2)), ha="center")
  if noux:
    plots = [sigmas3plot, corralplot]
  else:
    plots = [sigmaplot, corralplot]
  ax.set_ylim(bottom=0, top=max(sigma_times + sigmas3_times + corral_times)*1.2)
  plt.tick_params(
    axis='x',          # changes apply to the x-axis
    which='both',      # both major and minor ticks are affected
    bottom=False,      # ticks along the bottom edge are off
    top=False,         # ticks along the top edge are off
    labelbottom=False) # labels along the bottom edge are off
  #plt.xticks(sigmax + width, ("Cold-start", "Warm-start"))

  if app == "grep":
    title = "MapReduce Grep Execution Time"
  elif app == "wc":
    title = "MapReduce WordCount Execution Time"
  else:
    assert(False)

  finalize_graph(fig, ax, plots, title, out)

if __name__ == "__main__":
  parser = argparse.ArgumentParser()
  parser.add_argument("--measurement_dir", type=str, required=True)
  parser.add_argument("--datasize", type=str, required=True)
  parser.add_argument("--granular", action="store_true", default=False)
  parser.add_argument("--noux", action="store_true", default=False)
  parser.add_argument("--app", type=str, required=True)
  parser.add_argument("--out", type=str, required=True)

  args = parser.parse_args()
  graph_data(args.measurement_dir, args.app, args.datasize, args.granular, args.noux, args.out)
