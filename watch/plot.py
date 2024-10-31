#!/usr/bin/env python3

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from columnar import columnar

# TODO: if have time, make this connect directly to s3
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

def plot_histogram(data, bins=10, title="Histogram", xlabel="Value", ylabel="Frequency", save=True):
    plt.hist(data, bins=bins, edgecolor='black', alpha=0.4)
    plt.title(title)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.grid(True)
    if save:
        plt.savefig(f"{title}.png")

if __name__ == "__main__":
    create_watch_times,delete_watch_times = read_data(
        "watchperf_multiple_no_files_2024-10-31 07_38_06.427872582 +0000 UTC m=+73.577331401.txt"
    )

    create_watch_times = remove_outliers(create_watch_times)
    delete_watch_times = remove_outliers(delete_watch_times)

    print_stats(create_watch_times, delete_watch_times)
    
    plot_histogram(create_watch_times, bins=30, title="", xlabel="Delay (us)", ylabel="Frequency", save=False)
    plot_histogram(delete_watch_times, bins=30, title="Watch Times", xlabel="Delay (us)", ylabel="Frequency")
    plt.clf()
