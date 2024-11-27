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
    plt.hist(data, bins=bins, edgecolor='black', alpha=0.4, label=label)
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

if __name__ == "__main__":
    timestamp = "2024-11-27_03:53:28"
    session = boto3.Session(profile_name='sigmaos')
    s3_resource = session.resource('s3')
    bucket = s3_resource.Bucket('sigmaos-bucket-ryan')
            
    for v in ['v1', 'v2']:
        for loc in ['named', 'local']:
            for typ in ['include_op', 'watch_only']:
                process_file(f"{timestamp}/{v}/watchperf_single_no_files_{loc}_{typ}.txt", bucket, label_suffix=" (0 other files in dir)")
                process_file(f"{timestamp}/{v}/watchperf_single_some_files_{loc}_{typ}.txt", bucket, label_suffix=" (100 other files in dir)")
                process_file(f"{timestamp}/{v}/watchperf_single_many_files_{loc}_{typ}.txt", bucket, label_suffix=" (1000 other files in dir)")
                save_file(f"./{timestamp}/{v}/watchperf_single_{loc}_{typ}.png")

                process_file(f"{timestamp}/{v}/watchperf_multiple_no_files_{loc}_{typ}.txt", bucket, label_suffix=" (10 workers)")
                process_file(f"{timestamp}/{v}/watchperf_single_no_files_{loc}_{typ}.txt", bucket, label_suffix=" (1 workers)")
                save_file(f"./{timestamp}/{v}/watchperf_multiple_{loc}_{typ}.png")

