#!/usr/bin/env python3

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from columnar import columnar

def read_data(file_path):
    with open(file_path, 'r') as file:
        data = file.readlines()
    
    create_watch_times = data[0].strip().split(',')
    delete_watch_times = data[1].strip().split(',')

    create_watch_times = [int(x) / 1000.0 for x in create_watch_times if x]
    delete_watch_times = [int(x) / 1000.0 for x in delete_watch_times if x]

    return create_watch_times, delete_watch_times

def describe(times):
    series = pd.Series(times)
    print(series.describe())
    print()

def print_stats(create_times, delete_times):
    data = [
        ["Metric", "Create", "Delete"],
        ["Mean", np.mean(create_times), np.mean(delete_times)],
        ["Median", np.median(create_times), np.median(delete_times)],
        ["Std Dev", np.std(create_times), np.std(delete_times)],
        ["Min", np.min(create_times), np.min(delete_times)],
        ["Max", np.max(create_times), np.max(delete_times)]
    ]

    headers = data.pop(0)
    table = columnar(data, headers, no_borders=True)
    print(table)

def remove_outliers(data):
    Q1 = np.percentile(data, 25)
    Q3 = np.percentile(data, 75)
    
    IQR = Q3 - Q1
    
    lower_bound = Q1 - 1.5 * IQR
    upper_bound = Q3 + 1.5 * IQR
    
    return [x for x in data if lower_bound <= x <= upper_bound]

def plot_histogram(data, bins=10, title="Histogram", xlabel="Value", ylabel="Frequency", save=None, label=None):
    plt.hist(data, bins=bins, edgecolor='black', alpha=0.4, label=label)
    plt.title(title)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.grid(True)
    plt.legend()
    if save:
        plt.savefig(save)

def process_file(file, save=None, label_suffix=""):
    create_watch_times, delete_watch_times = read_data(file)
    if save == "":
        save = file.replace(".txt", ".png")

    create_watch_times = remove_outliers(create_watch_times)
    delete_watch_times = remove_outliers(delete_watch_times)

    print_stats(create_watch_times, delete_watch_times)
    
    plot_histogram(create_watch_times, bins=30, title="", xlabel="Delay (us)", ylabel="Frequency", save=save, label=("Create" + label_suffix))
    # plot_histogram(delete_watch_times, bins=30, title="Watch Times", xlabel="Delay (us)", ylabel="Frequency", save=save, label=("Delete" + label_suffix))

if __name__ == "__main__":
    process_file("./v1/watchperf_single_no_files_named_include_op.txt", label_suffix=" (No Files)")
    process_file("./v1/watchperf_single_some_files_named_include_op.txt", label_suffix=" (Some Files)")
    process_file("./v1/watchperf_single_many_files_named_include_op.txt", label_suffix=" (Many Files)", save="./v1/watchperf_single_named_include_op.png")
    plt.clf()

    process_file("./v1/watchperf_single_no_files_local_include_op.txt", label_suffix=" (No Files)")
    process_file("./v1/watchperf_single_some_files_local_include_op.txt", label_suffix=" (Some Files)")
    process_file("./v1/watchperf_single_many_files_local_include_op.txt", label_suffix=" (Many Files)", save="./v1/watchperf_single_local_include_op.png")
    plt.clf()

    process_file("./v1/watchperf_single_no_files_named_watch_only.txt", label_suffix=" (No Files)")
    process_file("./v1/watchperf_single_some_files_named_watch_only.txt", label_suffix=" (Some Files)")
    process_file("./v1/watchperf_single_many_files_named_watch_only.txt", label_suffix=" (Many Files)", save="./v1/watchperf_single_named_watch_only.png")
    plt.clf()

    process_file("./v1/watchperf_single_no_files_local_watch_only.txt", label_suffix=" (No Files)")
    process_file("./v1/watchperf_single_some_files_local_watch_only.txt", label_suffix=" (Some Files)")
    process_file("./v1/watchperf_single_many_files_local_watch_only.txt", label_suffix=" (Many Files)", save="./v1/watchperf_single_local_watch_only.png")
    plt.clf()
