#!/usr/bin/python3
import os
import subprocess
import argparse
import yaml
import numpy as np
from dateutil import parser as dateparser

def run_process_get_output(command):
    process = subprocess.Popen(command, stdout=subprocess.PIPE) 
    return str(process.communicate()[0]).replace('\\n', '\n')[2:-1]

def start_time_stats(depname):
    out = run_process_get_output(["kubectl", "get", "pods", "--all-namespaces"])
    pod_names = [ n for n in out.split() if depname in n ]
    pod_details = [ yaml.load(run_process_get_output(["kubectl", "get", "pods", pn, "-o", "yaml"])) for pn in pod_names ]
    pod_transitions = [ pd["status"]["conditions"] for pd in pod_details ]
    pod_scheduled_time_strs = [ [ t["lastTransitionTime"] for t in pts if t["type"] == "PodScheduled" ][0] for pts in pod_transitions ]
    pod_ready_time_strs = [ [ t["lastTransitionTime"] for t in pts if t["type"] == "Ready" ][0] for pts in pod_transitions ]
    pod_scheduled_times = [ dateparser.parse(s) for s in pod_scheduled_time_strs ]
    pod_ready_times = [ dateparser.parse(s) for s in pod_ready_time_strs ]
    pod_startup_times = [ (pod_ready_times[i] - pod_scheduled_times[i]).total_seconds() for i in range(len(pod_ready_times)) ]
    print("Mean pod startup time:", np.mean(pod_startup_times))
    print("Std dev pod startup time:", np.std(pod_startup_times))
    print(np.mean(pod_startup_times))
     
if __name__ == "__main__":                                                    
    parser = argparse.ArgumentParser()                                        
    parser.add_argument("--depname", type=str, required=True)
                                       
    args = vars(parser.parse_args())
                                       
    start_time_stats(**args) 
