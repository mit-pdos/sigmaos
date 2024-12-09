#!/usr/bin/env python3

import boto3
import os
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from columnar import columnar

def read_data(file_path, bucket):
    obj = bucket.Object(file_path)
    data = obj.get()['Body'].read().decode('utf-8').split('\n')
    
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

def plot_histogram(data, bins=10, title="Histogram", xlabel="Value", ylabel="Frequency", label=None):
    hist = plt.hist(data, bins=bins, edgecolor='black', alpha=0.4, label=label)
    hist_color = hist[2][0].get_facecolor()
    plt.axvline(np.mean(data), linestyle='dashed', linewidth=2, color=hist_color)

    plt.title(title)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.grid(True)
    plt.legend()

def process_file(file, bucket, label_suffix=""):
    create_watch_times, delete_watch_times = read_data(file, bucket)

    create_watch_times = remove_outliers(create_watch_times)
    delete_watch_times = remove_outliers(delete_watch_times)

    # print_stats(create_watch_times, delete_watch_times)
    
    plot_histogram(create_watch_times, bins=30, title="", xlabel="Delay (us)", ylabel="Frequency", label=("Create" + label_suffix))
    # plot_histogram(delete_watch_times, bins=30, title="Watch Times", xlabel="Delay (us)", ylabel="Frequency", save=save, label=("Delete" + label_suffix))

def save_file(save):
    os.makedirs(os.path.dirname(save), exist_ok=True)
    plt.savefig(save)
    plt.clf()

def plot_histograms(timestamp, bucket):
    for v in ['V1', 'V2']:
        for loc in ['named', 'local']:
            for typ in ['include_op', 'watch_only']:
                process_file(f"{timestamp}/{v}/1wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (0 other files)")
                process_file(f"{timestamp}/{v}/1wkrs_100stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (100 other files)")
                process_file(f"{timestamp}/{v}/1wkrs_500stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (500 other files)")
                process_file(f"{timestamp}/{v}/1wkrs_1000stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (1000 other files)")
                save_file(f"./{timestamp}/{v}/1wkrs_*stfi_1fpt_{loc}_{typ}.png")

                process_file(f"{timestamp}/{v}/1wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (1 watcher)")
                process_file(f"{timestamp}/{v}/5wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (5 watchers)")
                process_file(f"{timestamp}/{v}/10wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (10 watchers)")
                process_file(f"{timestamp}/{v}/15wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (15 watchers)")
                save_file(f"./{timestamp}/{v}/*wkrs_0stfi_1fpt_{loc}_{typ}.png")

                process_file(f"{timestamp}/{v}/1wkrs_0stfi_1fpt_{loc}_{typ}", bucket, label_suffix=" (1 file per trial)")
                process_file(f"{timestamp}/{v}/1wkrs_0stfi_5fpt_{loc}_{typ}", bucket, label_suffix=" (5 files per trial)")
                process_file(f"{timestamp}/{v}/1wkrs_0stfi_10fpt_{loc}_{typ}", bucket, label_suffix=" (10 files per trial)")
                process_file(f"{timestamp}/{v}/1wkrs_0stfi_15fpt_{loc}_{typ}", bucket, label_suffix=" (15 files per trial)")
                save_file(f"./{timestamp}/{v}/1wkrs_0stfi_*fpt_{loc}_{typ}.png")

def compute_speedups(timestamp, bucket):
    files = []
    for obj in bucket.objects.filter(Prefix=f"{timestamp}/V1/"):
        files.append(obj.key)

    speedups_include_op = []
    speedups_watch_only = []
    speedups_1000_stfi = []

    for file in files:
        if file.endswith('_'):
            continue
        v1_create, _ = read_data(file, bucket)
        v1_create = remove_outliers(v1_create)

        v2_create, _ = read_data(file.replace("V1", "V2"), bucket)
        v2_create = remove_outliers(v2_create)

        create_speedup = np.mean(v1_create) / np.mean(v2_create)

        print(f"{file} create speedup: {create_speedup}")

        if "include_op" in file:
            speedups_include_op.append(create_speedup)
        else:
            speedups_watch_only.append(create_speedup)
            if "1000stfi" in file:
                speedups_1000_stfi.append(create_speedup)
    
    print("Include Op")
    print(pd.Series(speedups_include_op).describe())
    print()
    print("Watch Only")
    print(pd.Series(speedups_watch_only).describe())
    print()
    print("1000 stfi, watch only")
    print(pd.Series(speedups_1000_stfi).describe())

def plot_starting_file_graph(timestamp, bucket):
    data = [[], []]
    x_values = [0, 100, 500, 1000]
    for ix, v in enumerate(['V1', 'V2']):
        for nstfi in x_values:
            file = f"{timestamp}/{v}/1wkrs_{nstfi}stfi_1fpt_local_watch_only"
            create_watch_times, _ = read_data(file, bucket)
            create_watch_times = remove_outliers(create_watch_times)

            data[ix].append(np.mean(create_watch_times) / 1000.0)

    plt.plot(x_values, data[0], label="V1", marker='o')
    plt.plot(x_values, data[1], label="V2", marker='o')
    plt.xlabel("Num Starting Files")
    plt.ylabel("Mean Watch Time (us)")
    plt.xticks(x_values)
    plt.grid(axis='x', which='major')
    plt.grid(axis='y')
    plt.legend()
    os.makedirs(timestamp, exist_ok=True)
    plt.savefig(f"./{timestamp}/mean_watch_time_vs_starting_files.png")
    plt.clf()

def plot_wkrs_graph(timestamp, bucket):
    data = [[], []]
    x_values = [1, 5, 10, 15]
    for ix, v in enumerate(['V1', 'V2']):
        for nwkrs in x_values:
            file = f"{timestamp}/{v}/{nwkrs}wkrs_0stfi_1fpt_local_watch_only"
            create_watch_times, _ = read_data(file, bucket)
            create_watch_times = remove_outliers(create_watch_times)

            data[ix].append(np.mean(create_watch_times) / 1000.0)

    plt.plot(x_values, data[0], label="V1", marker='o')
    plt.plot(x_values, data[1], label="V2", marker='o')
    plt.xlabel("Num Workers")
    plt.ylabel("Mean Watch Time (us)")
    plt.xticks(x_values)
    plt.grid(axis='x', which='major')
    plt.grid(axis='y')
    plt.legend()
    os.makedirs(timestamp, exist_ok=True)
    plt.savefig(f"./{timestamp}/mean_watch_time_vs_workers.png")
    plt.clf()

def plot_fpt_graph(timestamp, bucket):
    data = [[], []]
    x_values = [1, 5, 10, 15]
    for ix, v in enumerate(['V1', 'V2']):
        for fpt in x_values:
            file = f"{timestamp}/{v}/1wkrs_0stfi_{fpt}fpt_local_watch_only"
            create_watch_times, _ = read_data(file, bucket)
            create_watch_times = remove_outliers(create_watch_times)

            data[ix].append(np.mean(create_watch_times) / 1000.0)

    plt.plot(x_values, data[0], label="V1", marker='o')
    plt.plot(x_values, data[1], label="V2", marker='o')
    plt.xlabel("Files per Trial")
    plt.ylabel("Mean Watch Time (us)")
    plt.xticks(x_values)
    plt.grid(axis='x', which='major')
    plt.grid(axis='y')
    plt.legend()
    os.makedirs(timestamp, exist_ok=True)
    plt.savefig(f"./{timestamp}/mean_watch_time_vs_files_per_trial.png")
    plt.clf()

if __name__ == "__main__":
    timestamp = "2024-12-05_16:27:07"
    session = boto3.Session(profile_name='sigmaos')
    s3_resource = session.resource('s3')
    bucket = s3_resource.Bucket('sigmaos-bucket-ryan')
            
    # plot_histograms(timestamp, bucket)
    # compute_speedups(timestamp, bucket)
    plot_starting_file_graph(timestamp, bucket)
    plot_wkrs_graph(timestamp, bucket)
    plot_fpt_graph(timestamp, bucket)