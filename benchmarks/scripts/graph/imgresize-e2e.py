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

x =       [10, 20, 40, 80, 160, 320]
y_sigma = 1.0 / np.array([3.72, 4.49, 6.49, 13.54, 28.53, 55.64])
y_k8s =   1.0 / np.array([6.0, 11, 15, 37, 82, 117])
plt.rcParams["figure.figsize"] = (6.4,4.8)
plt.plot(x, y_sigma, label="SigmaOS", color="orange")
plt.plot(x, y_k8s, label="K8s", color="blue")
plt.xlabel("Number of image resizing tasks")
plt.ylabel("Task throughput (tasks/sec)")
plt.legend(bbox_to_anchor=(1.02, .5), loc="center left")
plt.savefig("/home/arielck/sigmaos/benchmarks/results/graphs/imgresize_e2e.pdf", bbox_inches="tight")
